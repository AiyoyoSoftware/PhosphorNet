package main

import (
	"fmt"
	"os"

	"phosphornet/internal/client"
)

func main() {
	if err := client.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
