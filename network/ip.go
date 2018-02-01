package network

import (
	"net"
	"fmt"
	"io/ioutil"
	"net/http"
)

func MyIp() (net.IP, error) {
	res, err := http.Get("http://api.ipify.org") //TODO: Test ipv6
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("IP looking caused HTTP error: %d", res.StatusCode)
	}

	ips, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	v := string(ips)
	ip := net.ParseIP(v)

	if ip == nil {
		return nil, fmt.Errorf("%q could not be parsed as an IP address", v)
	}

	return ip, nil
}

func StringIps(ips []net.IP) string {
	return fmt.Sprintf("%s", ips)
}


func Contains(ips []net.IP, ip net.IP) bool {
	for _, i := range ips {
		if i.Equal(ip) {
			return true
		}
	}

	return false
}