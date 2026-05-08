package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"trish/buildcfg"
	"trish/server"
)

func main() {
	cfg := defaultRuntimeConfig()
	mode := "run-foreground"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		mode = args[0]
		args = args[1:]
	}

	applyArgs(&cfg, args)

	switch mode {
	case "install":
		runInstall(cfg)
	case "install-check":
		runInstallCheck(cfg)
	case "repair":
		runRepair(cfg)
	case "uninstall":
		runUninstall()
	case "run-service":
		if err := server.RunServiceMode(); err != nil {
			log.Fatal(err)
		}
	case "run-foreground":
		if err := runServerForeground(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("unknown mode: %s\n", mode)
		fmt.Println("modes: install, install-check, repair, uninstall, run-service, run-foreground")
		os.Exit(1)
	}
}

type runtimeConfig struct {
	Port         int    `json:"port"`
	RegistryPath string `json:"registry_path"`
	LockPath     string `json:"lock_path"`
	AdminSecret  string `json:"admin_secret"`
}

func defaultRuntimeConfig() runtimeConfig {
	cfg := runtimeConfig{
		Port:         mustAtoi(buildcfg.DefaultServerPort, 9999),
		RegistryPath: defaultRegistryPath(),
		LockPath:     defaultLockPath(),
		AdminSecret:  buildcfg.DefaultAdminSecret,
	}
	if strings.TrimSpace(buildcfg.DefaultServerRegistry) != "" {
		cfg.RegistryPath = buildcfg.DefaultServerRegistry
	}
	if strings.TrimSpace(buildcfg.DefaultServerLock) != "" {
		cfg.LockPath = buildcfg.DefaultServerLock
	}
	return cfg
}

func applyArgs(cfg *runtimeConfig, args []string) {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-port="), strings.HasPrefix(arg, "--port="):
			value := arg[strings.Index(arg, "=")+1:]
			parsed, err := strconv.Atoi(value)
			if err != nil {
				log.Fatalf("invalid port: %s", value)
			}
			cfg.Port = parsed
		case strings.HasPrefix(arg, "-registry="), strings.HasPrefix(arg, "--registry="):
			cfg.RegistryPath = arg[strings.Index(arg, "=")+1:]
		case strings.HasPrefix(arg, "-lock="), strings.HasPrefix(arg, "--lock="):
			cfg.LockPath = arg[strings.Index(arg, "=")+1:]
		case strings.HasPrefix(arg, "-admin-secret="), strings.HasPrefix(arg, "--admin-secret="):
			cfg.AdminSecret = arg[strings.Index(arg, "=")+1:]
		}
	}
}

func runServerForeground(cfg runtimeConfig) error {
	_, _, release, err := startServer(cfg)
	if err != nil {
		log.Printf("server startup error: %v", err)
		fmt.Println("La fenetre restera ouverte 10 secondes pour laisser lire l'erreur.")
		time.Sleep(10 * time.Second)
		os.Exit(1)
	}
	defer release()

	fmt.Println("=== Trish Server ===")
	fmt.Printf("Listening on :%d\n", cfg.Port)
	fmt.Printf("Registry: %s\n", cfg.RegistryPath)
	fmt.Printf("Lock: %s\n", cfg.LockPath)
	select {}
}

func startServer(cfg runtimeConfig) (*server.ProcessLock, *server.Server, func(), error) {
	lock, err := server.AcquireProcessLock(cfg.LockPath)
	if err != nil {
		return nil, nil, nil, err
	}

	srv, err := server.NewServer(cfg.Port, cfg.RegistryPath, cfg.AdminSecret)
	if err != nil {
		lock.Release()
		return nil, nil, nil, err
	}

	if err := srv.Start(); err != nil {
		lock.Release()
		return nil, nil, nil, err
	}

	release := func() {
		srv.Stop()
		lock.Release()
	}
	return lock, srv, release, nil
}

func runInstall(cfg runtimeConfig) {
	fmt.Println("=== Trish Server Setup ===")
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fatalServer(err)
	}
	if err := server.InstallOrRepairService(server.ServiceInstallOptions{
		Port:         cfg.Port,
		RegistryPath: cfg.RegistryPath,
		LockPath:     cfg.LockPath,
		AdminSecret:  cfg.AdminSecret,
		ConfigData:   configData,
	}, progress); err != nil {
		fatalServer(err)
	}
}

func runInstallCheck(cfg runtimeConfig) {
	fmt.Println("=== Trish Server Install Check ===")
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fatalServer(err)
	}
	if err := server.InstallCheckService(server.ServiceInstallOptions{
		Port:         cfg.Port,
		RegistryPath: cfg.RegistryPath,
		LockPath:     cfg.LockPath,
		AdminSecret:  cfg.AdminSecret,
		ConfigData:   configData,
	}, progress); err != nil {
		fatalServer(err)
	}
}

func runRepair(cfg runtimeConfig) {
	fmt.Println("=== Trish Server Repair ===")
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fatalServer(err)
	}
	if err := server.InstallOrRepairService(server.ServiceInstallOptions{
		Port:         cfg.Port,
		RegistryPath: cfg.RegistryPath,
		LockPath:     cfg.LockPath,
		AdminSecret:  cfg.AdminSecret,
		ConfigData:   configData,
	}, progress); err != nil {
		fatalServer(err)
	}
}

func runUninstall() {
	fmt.Println("=== Trish Server Uninstall ===")
	if err := server.UninstallService(progress); err != nil {
		fatalServer(err)
	}
}

func progress(format string, args ...interface{}) {
	fmt.Printf("- "+format+"\n", args...)
}

func fatalServer(err error) {
	log.Printf("server error: %v", err)
	fmt.Println("La fenetre restera ouverte 10 secondes pour laisser lire l'erreur.")
	time.Sleep(10 * time.Second)
	os.Exit(1)
}

func defaultRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "registry.json"
	}
	return filepath.Join(home, ".trish", "registry.json")
}

func defaultLockPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "server.lock"
	}
	return filepath.Join(home, ".trish", "server.lock")
}

func mustAtoi(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
