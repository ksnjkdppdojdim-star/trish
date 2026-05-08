// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

//go:build !windows

package agent

func IsAdminRequired(err error) bool {
	return false
}

func RelaunchElevated(args []string) error {
	return nil
}
