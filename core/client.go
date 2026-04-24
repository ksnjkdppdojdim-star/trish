package core

import (
	"fmt"
	"net"
	"time"
)

// Client represente le client admin qui parle au serveur central.
type Client struct {
	ID         string
	ServerAddr string
	ServerPort int
	Timeout    time.Duration
}

// NewClient cree un client connecte au serveur Trish.
func NewClient(serverAddr string, serverPort int) *Client {
	if serverAddr == "" {
		serverAddr = "127.0.0.1"
	}
	if serverPort == 0 {
		serverPort = 9999
	}

	return &Client{
		ID:         fmt.Sprintf("client-%d", time.Now().UnixNano()),
		ServerAddr: serverAddr,
		ServerPort: serverPort,
		Timeout:    5 * time.Second,
	}
}

func (c *Client) do(msg *Message) (*Message, error) {
	addr := fmt.Sprintf("%s:%d", c.ServerAddr, c.ServerPort)
	conn, err := net.DialTimeout("tcp", addr, c.Timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return nil, err
	}

	encoder := NewEncoder(conn)
	decoder := NewDecoder(conn)

	if msg.RequestID == "" {
		msg.RequestID = NewRequestID("cli")
	}

	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	var resp Message
	if err := decoder.Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	return &resp, nil
}

// ListAgents retourne la liste des agents connus du serveur.
func (c *Client) ListAgents() ([]AgentRegistryEntry, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIList})
	if err != nil {
		return nil, err
	}
	return resp.Agents, nil
}

// GetAgent retourne les informations d'un agent.
func (c *Client) GetAgent(agentID string) (*AgentRegistryEntry, []string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIInfo, AgentID: agentID})
	if err != nil {
		return nil, nil, err
	}
	return resp.Agent, resp.Commands, nil
}

// ExecuteOnAgent execute une commande via le serveur central.
func (c *Client) ExecuteOnAgent(agentID string, cmdName string, args []string) (string, error) {
	resp, err := c.do(&Message{
		Type:    MessageTypeCLIExec,
		AgentID: agentID,
		Command: cmdName,
		Args:    args,
	})
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", fmt.Errorf(resp.Error)
	}
	return resp.Result, nil
}

// PingAgent verifie qu'un agent repond via le serveur.
func (c *Client) PingAgent(agentID string) (string, error) {
	resp, err := c.do(&Message{
		Type:    MessageTypeCLIPing,
		AgentID: agentID,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

// ControlAgent envoie une action d'administration a un agent.
func (c *Client) ControlAgent(agentID, control string) (string, error) {
	resp, err := c.do(&Message{
		Type:    MessageTypeCLIAgentControl,
		AgentID: agentID,
		Control: control,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}
