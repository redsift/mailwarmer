package smtp

import (
	"strings"
	"golang.org/x/net/idna"
	"errors"
)

func LocalAndDomainForEmailAddress(raw string) (string, string, error) {
	addr := strings.TrimSpace(raw)

	at := strings.LastIndex(addr, "@")
	var local, domain string
	if at < 0 {
		domain = addr
	} else {
		local, domain = addr[:at], addr[at+1:]
	}
	domain, err := idna.ToASCII(domain)
	if err != nil {
		return "", "", err
	}

	if domain == "" {
		return "", "", errors.New("malformed address")
	}

	return local, strings.ToLower(domain), nil
}
