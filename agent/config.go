// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ServiceName        = "TrishAgent"
	ServiceDisplayName = "Trish Agent"
)

// Config represente la configuration persistante de l'agent.
type Config struct {
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	ListenPort int    `json:"listen_port"`
	InstallDir string `json:"install_dir"`
	LogDir     string `json:"log_dir"`
	Version    string `json:"version"`
}

// DefaultInstallDir retourne le dossier d'installation officiel.
func DefaultInstallDir() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
	}
	return filepath.Join(programData, "Trish", "agent")
}

// DefaultConfigPath retourne le chemin du fichier de configuration.
func DefaultConfigPath() string {
	return filepath.Join(DefaultInstallDir(), "agent-config.json")
}

// DefaultLogDir retourne le dossier de logs officiel.
func DefaultLogDir() string {
	return filepath.Join(DefaultInstallDir(), "logs")
}

// DefaultExecutablePath retourne le chemin cible du binaire installe.
func DefaultExecutablePath() string {
	return filepath.Join(DefaultInstallDir(), "trish-agent.exe")
}

// LoadConfig lit la configuration agent.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save ecrit la configuration sur disque.
func (c *Config) Save(path string) error {
	if c.InstallDir == "" {
		c.InstallDir = DefaultInstallDir()
	}
	if c.LogDir == "" {
		c.LogDir = DefaultLogDir()
	}
	if c.Version == "" {
		c.Version = agentVersion
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Validate verifie la coherence de base de la configuration.
func (c *Config) Validate() error {
	if c.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}
	if c.ServerPort <= 0 {
		return fmt.Errorf("server port is invalid")
	}
	if c.ListenPort <= 0 {
		c.ListenPort = 2222
	}
	return nil
}
