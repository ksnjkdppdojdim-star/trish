package agent

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"trish/core"
	"trish/plugins/cd"
	"trish/plugins/dir"
	"trish/plugins/ipconfig"
)

// RegisterDefaultPlugins enregistre les plugins de base.
func RegisterDefaultPlugins(agentSvc *AgentService) {
	session := core.NewSessionState()
	_ = agentSvc.Agent().Registry.Register(&ipconfig.IpconfigCommand{})
	_ = agentSvc.Agent().Registry.Register(cd.NewCdCommand(session))
	_ = agentSvc.Agent().Registry.Register(dir.NewDirCommand(session))
}

// StartWithConfig demarre l'agent avec une configuration.
func StartWithConfig(cfg *Config, logger *log.Logger) (*AgentService, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	agentSvc := NewAgentService(cfg.ServerAddr, cfg.ServerPort, cfg.ListenPort)
	RegisterDefaultPlugins(agentSvc)
	if err := agentSvc.Start(); err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Printf("agent started version=%s server=%s:%d", cfg.Version, cfg.ServerAddr, cfg.ServerPort)
	}

	return agentSvc, nil
}

// OpenLogger ouvre le fichier de log officiel de l'agent.
func OpenLogger(cfg *Config) (*log.Logger, io.Closer, error) {
	if cfg.LogDir == "" {
		cfg.LogDir = DefaultLogDir()
	}
	if err := os.MkdirAll(cfg.LogDir, 0700); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(cfg.LogDir, "agent.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, err
	}

	return log.New(file, "trish-agent ", log.LstdFlags|log.LUTC), file, nil
}
