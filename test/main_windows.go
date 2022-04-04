package main

import "fmt"

func main() {
	var b byte
	fmt.Println("press any key to continue")
	_, _ = fmt.Scan(&b)

	testUDP()
	testTCP()
}
