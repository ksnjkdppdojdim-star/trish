// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package ipconfig

import (
	"fmt"
	"net"
	"os"
	"runtime"
)

// IpconfigCommand implémente la commande ipconfig
type IpconfigCommand struct{}

func (ic *IpconfigCommand) Name() string {
	return "ipconfig"
}

func (ic *IpconfigCommand) Description() string {
	return "Display network configuration"
}

func (ic *IpconfigCommand) Execute(args []string) (string, error) {
	var result string

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get interfaces: %v", err)
	}

	result += fmt.Sprintf("System: %s %s\n", runtime.GOOS, runtime.GOARCH)
	result += fmt.Sprintf("Hostname: %s\n\n", getHostname())
	result += "Network Interfaces:\n"
	result += "===================\n"

	for _, iface := range interfaces {
		result += fmt.Sprintf("\nInterface: %s\n", iface.Name)
		result += fmt.Sprintf("  Status: %s\n", iface.Flags)
		result += fmt.Sprintf("  MTU: %d\n", iface.MTU)
		result += fmt.Sprintf("  MAC: %s\n", iface.HardwareAddr)

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			result += fmt.Sprintf("  Address: %s\n", addr.String())
		}
	}

	return result, nil
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
