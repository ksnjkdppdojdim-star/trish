// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package main

import (
	"os"
	"trish/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
