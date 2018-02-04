package main

import (
	"github.com/alecthomas/kingpin"
	"golang.org/x/time/rate"
	"time"
	"math"
	"fmt"
	"context"
	"go.uber.org/zap"
	"github.com/redsift/mailwarmer/smtp"
	"net/mail"
	"io/ioutil"
	"github.com/redsift/mailwarmer/network"
	"github.com/miekg/dns"
)

const (
	EnvMXBind   = "MX_BIND"
	EnvSMTPBind = "SMTP_BIND"
	EnvMinRate = "SEND_MIN"
	EnvMaxRate = "SEND_MAX"
	EnvRateCoefficient = "SEND_COEF"
	EnvTimeOffset = "SEND_OFFSET"
	EnvSimuate = "SIMULATE"
	EnvEhlo = "EHLO"
)

var (
	Tag    = ""
	Commit = ""

	pSMTPBind  = kingpin.Flag("smtp-bind", "Bind the sender service to").Default("").Envar(EnvSMTPBind).String()
	pMxBind    = kingpin.Flag("mx-bind", "Bind the receiver service to").Default("0.0.0.0:25").Envar(EnvMXBind).String()

	pTimeOffset    = kingpin.Flag("send-time-offset", "Start the ramp from a future time").Default("24h").Envar(EnvTimeOffset).Duration()

	pMinRate    = kingpin.Flag("send-min-rate", "Minimum send rate per day").Default("50").Envar(EnvMinRate).Float64()
	pCoefficient    = kingpin.Flag("send-coefficient", "Exponent multiplication factor").Default("40").Envar(EnvRateCoefficient).Float64()
	pMaxRate    = kingpin.Flag("send-max-rate", "Maximum send rate per day").Default("2000").Envar(EnvMaxRate).Float64()


	pEhlo = kingpin.Flag("ehlo", "SMTP ehlo. Either the FQDN or the address literal e.g. [192.0.2.1] or [IPv6:fe80::1]").Default("").Envar(EnvEhlo).String()
	pSimuate    = kingpin.Flag("simulate", "Do not send emails").Short('s').Default("false").Envar(EnvSimuate).Bool()

)

const (
	EveryPeriod = time.Hour * 24
)

func rateForT(seconds float64) float64 {
	day := seconds / EveryPeriod.Seconds()

	r := *pCoefficient * math.Exp(day/2.0)

	if v := *pMinRate; r < v {
		r = v
	}

	if v := *pMaxRate; r > v {
		r = v
	}

	return r
}

func limit(t float64) (rate.Limit, time.Duration) {
	if t == 0 {
		t = 0.001
	}
	i := time.Duration(EveryPeriod.Seconds()/t)*time.Second

	return rate.Every(i), i
}

func main() {
	start := time.Now()
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	version := "unknown"
	if Tag == "" {
		if Commit != "" {
			version = Commit
		}
	} else {
		version = fmt.Sprintf("%s-%s", Tag, Commit)
	}
	kingpin.Version(version)
	kingpin.Parse()



	ip, err := network.MyIp()
	if err != nil {
		panic(err)
	}

	ptr, err := network.ReverseLookup(ip)
	if err != nil {
		panic(err)
	}

	logger.Info("Starting mailwarmer", zap.String("version", version), zap.Stringer("ip", ip), zap.String("ptr", ptr))

	ehlo := *pEhlo
	if ehlo == "" {
		ehlo = ptr
	} else {
		ehlo = dns.Fqdn(ehlo)
	}

	fwd, err := network.ForwardLookupA(ptr) //TODO: ipv6
	if err != nil || len(fwd) == 0 {
		logger.Warn("Deliverability issue detected, no A or AAAA record set for sender", zap.String("fqdn", ptr))
	} else 	if !network.Contains(fwd, ip) {
		logger.Warn("Deliverability issue detected, FCrDNS does not match", zap.Stringer("forward", ip), zap.String("reverse", network.StringIps(fwd))) //TODO: Point at URL
	} else if ehlo != ptr {
		logger.Warn("Deliverability issue detected, EHLO parameter should match PTR record")
	}

	sender, err := smtp.New(logger, ehlo, mail.Address{Name: "Magic", Address: "magic@test.com"})
	if err != nil {
		panic(err)
	}

	key, err := ioutil.ReadFile("./privkey.pem")
	if err != nil {
		panic(err)
	}
	sender.SetDKIM("warm", key)


	ctx := context.Background()

	var rt float64
	v, _ := limit(rt)
	limiter := rate.NewLimiter(v, 1)
	i := 0
	e := 0
	for {
		t := time.Now().Sub(start) + *pTimeOffset

		if r := rateForT(t.Seconds()); r != rt {
			rt = r
			v, n := limit(rt)
			limiter.SetLimit(v)
			logger.Info("Using rate per/day", zap.Float64("rate", rt), zap.Duration("now", t), zap.Duration("next", n))
		}

		limiter.Wait(ctx)

		to := mail.Address{Name: "Rahul Powar", Address: "rahul@redsift.io"}

		if *pSimuate {
			i++
			logger.Info("Simulating only", zap.Int("sent", i))
			continue
		}

		if err := sender.Send(ctx, to); err != nil {
			e++
			logger.Error("Send failed", zap.Error(err), zap.Int("errors", e))
		} else {
			i++
			logger.Info("Sent email", zap.Int("sent", i), zap.Int("errors", e))
		}
	}
}