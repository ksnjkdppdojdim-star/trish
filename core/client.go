package core

import (
	"fmt"
	"net"
	"strings"
	"time"
	"trish/buildcfg"
)

// Client represente le client admin qui parle au serveur central.
type Client struct {
	ID          string
	ServerAddr  string
	ServerPort  int
	Timeout     time.Duration
	AdminSecret string
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
		ID:          fmt.Sprintf("client-%d", time.Now().UnixNano()),
		ServerAddr:  serverAddr,
		ServerPort:  serverPort,
		Timeout:     5 * time.Second,
		AdminSecret: buildcfg.DefaultAdminSecret,
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
	if err := c.signMessage(msg); err != nil {
		return nil, err
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

func (c *Client) signMessage(msg *Message) error {
	if strings.TrimSpace(c.AdminSecret) == "" {
		return fmt.Errorf("admin secret is not configured")
	}
	nonce, err := NewAuthNonce()
	if err != nil {
		return err
	}

	msg.AuthClientID = c.ID
	msg.AuthTimestamp = time.Now().UTC().Unix()
	msg.AuthNonce = nonce
	return SignMessage(msg, c.AdminSecret)
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

func (c *Client) ListPlugins() ([]DynamicPluginManifest, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIPluginList})
	if err != nil {
		return nil, err
	}
	return resp.Plugins, nil
}

func (c *Client) InstallPlugin(pkg *DynamicPluginPackage) (string, error) {
	resp, err := c.do(&Message{
		Type:   MessageTypeCLIPluginInstall,
		Plugin: pkg,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) RemovePlugin(name string) (string, error) {
	resp, err := c.do(&Message{
		Type:       MessageTypeCLIPluginRemove,
		PluginName: name,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) EnablePlugin(name string) (string, error) {
	resp, err := c.do(&Message{
		Type:       MessageTypeCLIPluginEnable,
		PluginName: name,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) DisablePlugin(name string) (string, error) {
	resp, err := c.do(&Message{
		Type:       MessageTypeCLIPluginDisable,
		PluginName: name,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) ListPluginVersions(name string) ([]DynamicPluginManifest, error) {
	resp, err := c.do(&Message{
		Type:       MessageTypeCLIPluginVersions,
		PluginName: name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Plugins, nil
}

func (c *Client) RollbackPlugin(name string, version string) (string, error) {
	resp, err := c.do(&Message{
		Type:          MessageTypeCLIPluginRollback,
		PluginName:    name,
		PluginVersion: version,
	})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) ListGroups() ([]string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIGroupList})
	if err != nil {
		return nil, err
	}
	return resp.Groups, nil
}

func (c *Client) CreateGroup(name string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIGroupCreate, GroupName: name})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) DeleteGroup(name string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIGroupDelete, GroupName: name})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) AddAgentToGroup(agentID string, group string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIGroupAdd, AgentID: agentID, GroupName: group})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) RemoveAgentFromGroup(agentID string, group string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLIGroupRemove, AgentID: agentID, GroupName: group})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) SetAgentTags(agentID string, tags []string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLITagSet, AgentID: agentID, Tags: tags})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) AddAgentTags(agentID string, tags []string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLITagAdd, AgentID: agentID, Tags: tags})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}

func (c *Client) RemoveAgentTags(agentID string, tags []string) (string, error) {
	resp, err := c.do(&Message{Type: MessageTypeCLITagRemove, AgentID: agentID, Tags: tags})
	if err != nil {
		return "", err
	}
	return resp.Result, nil
}
