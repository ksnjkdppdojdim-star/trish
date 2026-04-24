package main

import (
	"os"
	"trish/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
