package smtp

import (
	"go.uber.org/zap"
	"net"
	"net/smtp"
	"context"
	"time"
	"errors"
	"net/mail"
	"crypto/tls"
	"github.com/toorop/go-dkim"
)

type sender struct {
	logger *zap.Logger
	dialer *net.Dialer

	elo string
	from mail.Address
	fromHost string

	dkimSelector string
	privateKey []byte
}

func New(logger *zap.Logger, elo string, from mail.Address) (*sender, error) {
	dialer := net.Dialer{Timeout: time.Second * 10}

	_, fromHost, err := LocalAndDomainForEmailAddress(from.Address)
	if err != nil {
		return nil, err
	}
	return &sender{logger: logger, dialer: &dialer, elo: elo, from: from, fromHost: fromHost}, nil
}

func (s *sender) SetDKIM(selector string, privateKey []byte) {
	s.dkimSelector = selector
	s.privateKey = privateKey
}

func (s *sender) Send(ctx context.Context, to mail.Address) error {
	s.logger.Debug("Sending...")

	msg := []byte("From: Magic <magic@test.com>\r\nTo: R <rahul@redsift.io>\r\nSubject: Testing\r\n\r\nHello, you want testing")
	return s.sendTo(ctx, to, msg)
}

func (s *sender) sendTo(ctx context.Context, to mail.Address, msg []byte) error {

	who, host, err := LocalAndDomainForEmailAddress(to.Address)
	if err != nil {
		return errors.New("Malformed host")
	}

	mxs, err := net.LookupMX(host)
	if err != nil {
		return errors.New("Unknown host")
	}

	if len(mxs) == 0 {
		return errors.New("No MX for host")
	}

	mx := mxs[0].Host

	conn, err := s.dialer.DialContext(ctx, "tcp", mx + ":smtp")
	if err != nil {
		return err
	}

	if s.dkimSelector != "" {

		options := dkim.NewSigOptions()
		options.PrivateKey = s.privateKey
		options.Domain = s.fromHost
		options.Selector = s.dkimSelector
		options.SignatureExpireIn = 3600
		options.BodyLength = 0 // TODO: uint(len(msg))

		// From:From:Subject:Subject:Date:To:To:MIME-Version:Content-Type
		// via https://wordtothewise.com/2014/05/dkim-injected-headers/
		//options.Headers = []string{"from", "subject"}
		options.Headers = []string{"from", "from", "subject", "subject", "date", "to", "to", "mime-version", "content-type", "return-path", "in-reply-to", "references", "cc"}
		options.AddSignatureTimestamp = true
		options.Canonicalization = "relaxed/relaxed"
		if err := dkim.Sign(&msg, options); err != nil {
			return err
		}
	}

	c := make(chan error, 1)
	var client *smtp.Client

	go func() {
		defer close(c)

		// Connect to the SMTP server
		client, err = smtp.NewClient(conn, mx)
		if err != nil {
			c <- err
			return
		}

		elo := s.elo
		if elo == "" {
			elo = "localhost"
		}
		err = client.Hello(elo)
		if err != nil {
			c <- err
			return
		}

		if ok, _ := client.Extension("STARTTLS"); ok {
			config := &tls.Config{ServerName: mx}
			if err = client.StartTLS(config); err != nil {
				c <- err
				return
			}
		}

		// TODO: Extension SMTPUTF8 is not supported
		err = client.Mail(s.from.Address)
		if err != nil {
			c <- err
			return
		}

		ina := who + "@" + host
		err = client.Rcpt(ina)
		if err != nil {
			c <- err
			return
		}

		w, err := client.Data()
		if err != nil {
			c <- err
			return
		}
		_, err = w.Write(msg)
		if err != nil {
			c <- err
			return
		}
		err = w.Close()
		if err != nil {
			c <- err
			return
		}
	}()

	select {
	case <-ctx.Done():
		defer func() {
			if client != nil {
				go client.Close()
			}
		}()
		return ctx.Err()
	case err := <-c:
		defer func() {
			if client != nil {
				go client.Quit()
			}
		}()
		return err
	}
}