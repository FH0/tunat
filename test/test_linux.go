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
		[]string{
			"ip tuntap add mode tun tun1 || true",
		},
		[]string{
			"ip link set tun1 up",
			"ip addr replace 10.0.0.1/24 dev tun1",
			"ip addr replace fd::1/120 dev tun1",
		},
	)
	if err != nil {
		panic(err)
	}
}
