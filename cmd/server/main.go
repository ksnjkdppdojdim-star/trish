package main

import (
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
	port := mustAtoi(buildcfg.DefaultServerPort, 9999)
	registryPath := defaultRegistryPath()
	lockPath := defaultLockPath()
	if strings.TrimSpace(buildcfg.DefaultServerRegistry) != "" {
		registryPath = buildcfg.DefaultServerRegistry
	}
	if strings.TrimSpace(buildcfg.DefaultServerLock) != "" {
		lockPath = buildcfg.DefaultServerLock
	}

	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "-port="), strings.HasPrefix(arg, "--port="):
			value := arg[strings.Index(arg, "=")+1:]
			parsed, err := strconv.Atoi(value)
			if err != nil {
				log.Fatalf("invalid port: %s", value)
			}
			port = parsed
		case strings.HasPrefix(arg, "-registry="), strings.HasPrefix(arg, "--registry="):
			registryPath = arg[strings.Index(arg, "=")+1:]
		case strings.HasPrefix(arg, "-lock="), strings.HasPrefix(arg, "--lock="):
			lockPath = arg[strings.Index(arg, "=")+1:]
		}
	}

	lock, err := server.AcquireProcessLock(lockPath)
	if err != nil {
		log.Printf("server startup error: %v", err)
		fmt.Println("La fenetre restera ouverte 10 secondes pour laisser lire l'erreur.")
		time.Sleep(10 * time.Second)
		os.Exit(1)
	}
	defer lock.Release()

	srv, err := server.NewServer(port, registryPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Trish Server ===")
	fmt.Printf("Listening on :%d\n", port)
	fmt.Printf("Registry: %s\n", registryPath)
	fmt.Printf("Lock: %s\n", lockPath)
	select {}
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
