package main

import (
	"fmt"
	"os"

	"github.com/0stick/CodeWhy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if !cli.ErrorWasRendered(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
