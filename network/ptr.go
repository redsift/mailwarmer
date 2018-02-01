package network

import (
	"net"
	"strconv"
	"github.com/miekg/dns"
	"errors"
	"time"
)

const hexDigit = "0123456789abcdef"
const timeout = time.Second * 5
const dnsServer ="8.8.8.8:53"
func reverseaddr(ip net.IP) (arpa string) {
	if ip.To4() != nil {
		return strconv.Itoa(int(ip[15])) + "." + strconv.Itoa(int(ip[14])) + "." + strconv.Itoa(int(ip[13])) + "." + strconv.Itoa(int(ip[12])) + ".in-addr.arpa."
	}

	// Must be IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}

	buf = append(buf, "ip6.arpa."...)
	return string(buf)
}

func ReverseLookup(ip net.IP) (string, error) {
	arpa := reverseaddr(ip)

	c := new(dns.Client)
	c.DialTimeout = timeout
	c.ReadTimeout = timeout
	c.WriteTimeout = timeout

	m := new(dns.Msg)
	m.SetQuestion(arpa, dns.TypePTR)
	m.RecursionDesired = true
	r, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return "", err
	}
	if r.Rcode != dns.RcodeSuccess {
		return "", errors.New("dns-ptr-failed")
	}
	for _, a := range r.Answer {
		if ptr, ok := a.(*dns.PTR); ok {
			return ptr.Ptr, nil
		}
	}
	return "", nil
}

func ForwardLookupA(domain string) ([]net.IP, error) {
	d := dns.Fqdn(domain)

	c := new(dns.Client)
	c.DialTimeout = timeout
	c.ReadTimeout = timeout
	c.WriteTimeout = timeout

	m := new(dns.Msg)
	m.SetQuestion(d, dns.TypeA)
	m.RecursionDesired = true
	r, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return nil, err
	}
	if r.Rcode != dns.RcodeSuccess {
		return nil, errors.New("dns-a-failed")
	}

	res := make([]net.IP, 0, len(r.Answer))
	for _, a := range r.Answer {
		if ptr, ok := a.(*dns.A); ok {
			res = append(res, ptr.A)
		}
	}

	return res, nil
}

func ForwardLookupAAAA(domain string) ([]net.IP, error) {
	d := dns.Fqdn(domain)

	c := new(dns.Client)
	c.DialTimeout = timeout
	c.ReadTimeout = timeout
	c.WriteTimeout = timeout

	m := new(dns.Msg)
	m.SetQuestion(d, dns.TypeA)
	m.RecursionDesired = true
	r, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return nil, err
	}
	if r.Rcode != dns.RcodeSuccess {
		return nil, errors.New("dns-aaaa-failed")
	}

	res := make([]net.IP, 0, len(r.Answer))
	for _, a := range r.Answer {
		if ptr, ok := a.(*dns.AAAA); ok {
			res = append(res, ptr.AAAA)
		}
	}

	return res, nil
}

