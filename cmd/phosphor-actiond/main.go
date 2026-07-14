package main

import (
	"fmt"
	"os"

	"phosphornet/internal/action"
)

func main() {
	if err := action.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
