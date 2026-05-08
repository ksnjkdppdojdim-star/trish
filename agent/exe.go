// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package agent

import "os"

func currentExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return exe
}
