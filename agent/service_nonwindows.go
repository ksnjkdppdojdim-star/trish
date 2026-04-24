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
