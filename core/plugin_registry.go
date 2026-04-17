package core

import (
	"fmt"
	"sync"
)

// PluginCommand définit l'interface pour chaque commande plugin
type PluginCommand interface {
	Name() string
	Description() string
	Execute(args []string) (string, error)
}

// PluginRegistry gère le registre des plugins
type PluginRegistry struct {
	mu       sync.RWMutex
	commands map[string]PluginCommand
}

// NewPluginRegistry crée un nouveau registre
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		commands: make(map[string]PluginCommand),
	}
}

// Register ajoute une commande au registre
func (pr *PluginRegistry) Register(cmd PluginCommand) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if _, exists := pr.commands[cmd.Name()]; exists {
		return fmt.Errorf("command %s already registered", cmd.Name())
	}
	pr.commands[cmd.Name()] = cmd
	return nil
}

// Execute exécute une commande enregistrée
func (pr *PluginRegistry) Execute(name string, args []string) (string, error) {
	pr.mu.RLock()
	cmd, exists := pr.commands[name]
	pr.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("command %s not found", name)
	}
	return cmd.Execute(args)
}

// List retourne la liste de toutes les commandes
func (pr *PluginRegistry) List() []string {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	var cmds []string
	for name := range pr.commands {
		cmds = append(cmds, name)
	}
	return cmds
}

// GetCommand retourne une commande par nom
func (pr *PluginRegistry) GetCommand(name string) (PluginCommand, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	cmd, exists := pr.commands[name]
	if !exists {
		return nil, fmt.Errorf("command %s not found", name)
	}
	return cmd, nil
}
