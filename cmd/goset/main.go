package main

import (
	"fmt"
	"os"

	"github.com/fior512/goset/internal/cli"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "goset:", err)
		os.Exit(2)
	}

	fmt.Printf("%+v\n", cfg)
}
