package main

import (
	"fmt"
	"os"

	"phosphornet/internal/node"
)

func main() {
	if err := node.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
