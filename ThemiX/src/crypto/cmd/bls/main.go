package main

import (
	"flag"
	"fmt"

	"go.themix.io/crypto/bls"
)

func main() {
	n := flag.Int("n", 5, "number of nodes")
	th := flag.Int("t", 3, "number of shares")
	flag.Parse()
	// Crypto setup
	err := bls.GenerateBlsKey("./", *n, *th)
	if err != nil {
		fmt.Println(err)
	}
	_, _, err = bls.LoadBlsKey("./", *n, *th)
	if err != nil {
		fmt.Println(err)
	}
}
