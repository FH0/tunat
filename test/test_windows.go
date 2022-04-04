package main

import (
	"net/netip"

	"github.com/FH0/tunat"
)

var tn *tunat.Tunat

func init() {
	var err error
	tn, err = tunat.New(
		"tun1",
		netip.MustParsePrefix("10.0.0.1/24"),
		netip.MustParsePrefix("fd::1/120"),
		1500,
		[]string{},
		[]string{
			"netsh interface ip set address tun1 static 10.0.0.1 255.255.255.0",
			"netsh interface ipv6 set address tun1 fd::1/120",
		},
	)
	if err != nil {
		panic(err)
	}
}
