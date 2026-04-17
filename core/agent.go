package core

import (
	"fmt"
	"net"
	"os"
	"time"
)

// Agent représente un agent Trish sur une machine distante
type Agent struct {
	ID           string
	Hostname     string
	IPAddress    string
	Port         int
	Registry     *PluginRegistry
	EventBus     *EventBus
	Connected    bool
	LastHeartbeat time.Time
}

// NewAgent crée un nouvel agent
func NewAgent(hostname string) *Agent {
	actualHostname, _ := os.Hostname()
	
	return &Agent{
		ID:            generateAgentID(actualHostname),
		Hostname:      actualHostname,
		Port:          2222,
		Registry:      NewPluginRegistry(),
		EventBus:      NewEventBus(),
		Connected:     false,
		LastHeartbeat: time.Now(),
	}
}

// GetLocalIP retourne l'IP locale de la machine
func (a *Agent) GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	a.IPAddress = conn.LocalAddr().(*net.UDPAddr).IP.String()
	return a.IPAddress, nil
}

// Start démarre l'agent (initialisation)
func (a *Agent) Start() error {
	a.GetLocalIP()
	a.Connected = true
	a.LastHeartbeat = time.Now()

	a.EventBus.Publish(&Event{
		Type:      EventAgentConnected,
		Timestamp: time.Now(),
		Source:    a.ID,
		Message:   fmt.Sprintf("Agent started on %s (%s:%d)", a.Hostname, a.IPAddress, a.Port),
		Data:      map[string]interface{}{"hostname": a.Hostname, "ip": a.IPAddress},
	})

	return nil
}

// ExecuteCommand exécute une commande enregistrée
func (a *Agent) ExecuteCommand(cmdName string, args []string) (string, error) {
	result, err := a.Registry.Execute(cmdName, args)
	
	a.EventBus.Publish(&Event{
		Type:      EventCommandExecuted,
		Timestamp: time.Now(),
		Source:    a.ID,
		Message:   fmt.Sprintf("Executed: %s %v -> %v", cmdName, args, err),
		Data: map[string]interface{}{
			"command": cmdName,
			"args":    args,
			"error":   err != nil,
		},
	})

	return result, err
}

// generateAgentID crée un ID unique pour l'agent
func generateAgentID(hostname string) string {
	return fmt.Sprintf("%s", hostname)
}

// Heartbeat met à jour le timestamp de connexion
func (a *Agent) Heartbeat() {
	a.LastHeartbeat = time.Now()
}
