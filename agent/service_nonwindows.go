// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

//go:build !windows

package agent

import "fmt"

func RunServiceMode() error {
	return fmt.Errorf("windows service mode is only available on Windows")
}

func RunForeground(cfg *Config) error {
	_, err := StartWithConfig(cfg, nil)
	if err != nil {
		return err
	}
	select {}
}
