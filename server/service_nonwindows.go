//go:build !windows

package server

import "fmt"

type ServiceInstallOptions struct {
	Port         int
	RegistryPath string
	LockPath     string
	AdminSecret  string
	ConfigData   []byte
}

func RunServiceMode() error {
	return fmt.Errorf("windows service mode is only available on Windows")
}

func InstallOrRepairService(ServiceInstallOptions, func(string, ...interface{})) error {
	return fmt.Errorf("windows service installation is only available on Windows")
}

func InstallCheckService(ServiceInstallOptions, func(string, ...interface{})) error {
	return fmt.Errorf("windows service installation is only available on Windows")
}

func UninstallService(func(string, ...interface{})) error {
	return fmt.Errorf("windows service installation is only available on Windows")
}

func DefaultConfigPath() string {
	return ""
}

func SaveServiceConfig([]byte) error {
	return fmt.Errorf("windows service installation is only available on Windows")
}
