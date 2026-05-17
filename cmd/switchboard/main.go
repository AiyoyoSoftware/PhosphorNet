package main

import (
	"fmt"
	"os"

	"phosphornet/internal/relay"
)

func main() {
	if err := relay.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
