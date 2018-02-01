package network

import (
	"testing"
	"net"
)

func TestLookup(t *testing.T) {
	addr, err := ReverseLookup(net.ParseIP("82.145.37.197"))
	if err != nil {
		t.Fatal(err)
	}

	if addr != "server6.codedmedia.co.uk." {
		t.Fatal("Unexpected result", addr)
	}
}
