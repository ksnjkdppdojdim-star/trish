package core

import (
	"fmt"
	"time"
)

// Client représente le client terminal Trish (chez l'admin)
type Client struct {
	ID        string
	Agents    map[string]*Agent
	EventBus  *EventBus
	Connected bool
}

// NewClient crée un nouveau client
func NewClient() *Client {
	return &Client{
		ID:       fmt.Sprintf("client-%d", time.Now().Unix()),
		Agents:   make(map[string]*Agent),
		EventBus: NewEventBus(),
	}
}

// DiscoverAgents simule la découverte d'agents sur le réseau (mDNS placeholder)
func (c *Client) DiscoverAgents() ([]string, error) {
	var agentIDs []string
	for id := range c.Agents {
		agentIDs = append(agentIDs, id)
	}
	return agentIDs, nil
}

// ConnectToAgent se connecte à un agent spécifique
func (c *Client) ConnectToAgent(agentID string) error {
	agent, exists := c.Agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if !agent.Connected {
		return fmt.Errorf("agent %s is not connected", agentID)
	}

	c.EventBus.Publish(&Event{
		Type:      EventAgentConnected,
		Timestamp: time.Now(),
		Source:    c.ID,
		Message:   fmt.Sprintf("Connected to agent %s", agentID),
		Data:      map[string]interface{}{"agent_id": agentID},
	})

	return nil
}

// ExecuteOnAgent exécute une commande sur un agent distant
func (c *Client) ExecuteOnAgent(agentID string, cmdName string, args []string) (string, error) {
	agent, exists := c.Agents[agentID]
	if !exists {
		return "", fmt.Errorf("agent %s not found", agentID)
	}

	if !agent.Connected {
		return "", fmt.Errorf("agent %s is disconnected", agentID)
	}

	return agent.ExecuteCommand(cmdName, args)
}

// ListAgents retourne la liste des agents connus
func (c *Client) ListAgents() []string {
	var agents []string
	for id, agent := range c.Agents {
		if agent.Connected {
			agents = append(agents, fmt.Sprintf("%s (%s@%s:%d)", id, agent.Hostname, agent.IPAddress, agent.Port))
		}
	}
	return agents
}

// AddAgent ajoute un agent au client
func (c *Client) AddAgent(agent *Agent) {
	c.Agents[agent.ID] = agent
}
