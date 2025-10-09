package main

import (
	"fmt"
	"os"
)

// main.go contains the CLI entry point. It simply delegates to rootCmd and
// ensures that any execution error is surfaced with a non-zero exit status.

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
