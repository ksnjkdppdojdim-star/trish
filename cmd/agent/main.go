package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"trish/agent"
	"trish/buildcfg"
	"trish/core"
)

func main() {
	serverAddr := buildcfg.DefaultServerAddr
	serverPort := mustAtoi(buildcfg.DefaultServerPort, 9999, "default server port")
	listenPort := mustAtoi(buildcfg.DefaultAgentListenPort, 2222, "default listen port")
	mode := "install"
	allowLoopbackServer := strings.EqualFold(strings.TrimSpace(buildcfg.AllowLoopbackServer), "true")

	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		mode = args[0]
		args = args[1:]
	}

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-server="), strings.HasPrefix(arg, "--server="):
			serverAddr = arg[strings.Index(arg, "=")+1:]
		case strings.HasPrefix(arg, "-port="), strings.HasPrefix(arg, "--port="):
			value := arg[strings.Index(arg, "=")+1:]
			parsed, err := strconv.Atoi(value)
			if err != nil {
				log.Fatalf("invalid server port: %s", value)
			}
			serverPort = parsed
		case strings.HasPrefix(arg, "-listen="), strings.HasPrefix(arg, "--listen="):
			value := arg[strings.Index(arg, "=")+1:]
			parsed, err := strconv.Atoi(value)
			if err != nil {
				log.Fatalf("invalid listen port: %s", value)
			}
			listenPort = parsed
		case arg == "-allow-loopback-server" || arg == "--allow-loopback-server":
			allowLoopbackServer = true
		}
	}

	switch mode {
	case "install":
		runInstall(serverAddr, serverPort, listenPort, allowLoopbackServer)
	case "install-check":
		runInstallCheck(serverAddr, serverPort, listenPort, allowLoopbackServer)
	case "repair":
		runRepair(serverAddr, serverPort, listenPort, allowLoopbackServer)
	case "uninstall":
		runUninstall()
	case "run-service":
		if err := agent.RunServiceMode(); err != nil {
			log.Fatal(err)
		}
	case "run-foreground":
		devDir := ".trish-agent-dev"
		cfg := &agent.Config{
			ServerAddr: serverAddr,
			ServerPort: serverPort,
			ListenPort: listenPort,
			InstallDir: devDir,
			LogDir:     devDir + "\\logs",
			Version:    core.Version,
		}
		if err := agent.RunForeground(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("unknown mode: %s\n", mode)
		fmt.Println("modes: install, install-check, repair, uninstall, run-service, run-foreground")
		os.Exit(1)
	}
}

func runInstall(serverAddr string, serverPort int, listenPort int, allowLoopbackServer bool) {
	fmt.Println("=== Trish Agent Setup ===")
	if err := validateInstallServer(serverAddr, allowLoopbackServer); err != nil {
		fatalInstall(err)
	}
	if err := agent.InstallOrRepair(agent.InstallOptions{
		ServerAddr: serverAddr,
		ServerPort: serverPort,
		ListenPort: listenPort,
	}, progress); err != nil {
		if agent.IsAdminRequired(err) {
			fmt.Println("Demande d'elevation administrateur...")
			if elevateErr := agent.RelaunchElevated(os.Args[1:]); elevateErr == nil {
				os.Exit(0)
			}
		}
		fatalInstall(err)
	}
}

func runRepair(serverAddr string, serverPort int, listenPort int, allowLoopbackServer bool) {
	fmt.Println("=== Trish Agent Repair ===")
	if err := validateInstallServer(serverAddr, allowLoopbackServer); err != nil {
		fatalInstall(err)
	}
	if err := agent.InstallOrRepair(agent.InstallOptions{
		ServerAddr: serverAddr,
		ServerPort: serverPort,
		ListenPort: listenPort,
	}, progress); err != nil {
		if agent.IsAdminRequired(err) {
			fmt.Println("Demande d'elevation administrateur...")
			if elevateErr := agent.RelaunchElevated(os.Args[1:]); elevateErr == nil {
				os.Exit(0)
			}
		}
		fatalInstall(err)
	}
}

func runInstallCheck(serverAddr string, serverPort int, listenPort int, allowLoopbackServer bool) {
	fmt.Println("=== Trish Agent Install Check ===")
	if err := validateInstallServer(serverAddr, allowLoopbackServer); err != nil {
		fatalInstall(err)
	}
	if err := agent.InstallCheck(agent.InstallOptions{
		ServerAddr: serverAddr,
		ServerPort: serverPort,
		ListenPort: listenPort,
		DryRun:     true,
	}, progress); err != nil {
		if agent.IsAdminRequired(err) {
			fmt.Println("Demande d'elevation administrateur...")
			if elevateErr := agent.RelaunchElevated(os.Args[1:]); elevateErr == nil {
				os.Exit(0)
			}
		}
		fatalInstall(err)
	}
}

func runUninstall() {
	fmt.Println("=== Trish Agent Uninstall ===")
	if err := agent.Uninstall(progress); err != nil {
		if agent.IsAdminRequired(err) {
			fmt.Println("Demande d'elevation administrateur...")
			if elevateErr := agent.RelaunchElevated(os.Args[1:]); elevateErr == nil {
				os.Exit(0)
			}
		}
		log.Fatal(err)
	}
}

func progress(format string, args ...interface{}) {
	fmt.Printf("- "+format+"\n", args...)
}

func fatalInstall(err error) {
	log.Printf("installation error: %v", err)
	fmt.Println("La fenetre restera ouverte 10 secondes pour laisser lire l'erreur.")
	time.Sleep(10 * time.Second)
	os.Exit(1)
}

func validateInstallServer(serverAddr string, allowLoopbackServer bool) error {
	if allowLoopbackServer {
		return nil
	}

	switch strings.TrimSpace(strings.ToLower(serverAddr)) {
	case "", "127.0.0.1", "localhost", "::1":
		return fmt.Errorf("refusing to install agent with loopback server address %q; use -server=<admin-pc-ip> or pass --allow-loopback-server for local-only testing", serverAddr)
	default:
		return nil
	}
}

func mustAtoi(value string, fallback int, label string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		log.Printf("invalid %s %q, using fallback %d", label, value, fallback)
		return fallback
	}
	return parsed
}
