package main

import (
	"fmt"
	"net"
)

func main() {
	ip := net.ParseIP("10.0.0.1")
	fmt.Println(ip, len(ip), ip.To4())
}
