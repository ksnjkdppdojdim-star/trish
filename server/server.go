// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package server

import (
	"crypto/hmac"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"trish/core"
)

// Server represente le serveur Trish central.
type Server struct {
	mu          sync.RWMutex
	port        int
	adminSecret string
	authMu      sync.Mutex
	seenNonces  map[string]time.Time
	registry    *core.AgentRegistry
	plugins     *pluginStore
	groups      *groupStore
	sessions    *sessionManager
	listener    net.Listener
	quit        chan struct{}
}

// NewServer cree un nouveau serveur.
func NewServer(port int, registryPath string, adminSecret string) (*Server, error) {
	registry, err := core.NewAgentRegistry(registryPath)
	if err != nil {
		return nil, err
	}
	plugins, err := newPluginStore(defaultPluginStorePath(registryPath))
	if err != nil {
		return nil, err
	}
	groups, err := newGroupStore(defaultGroupStorePath(registryPath))
	if err != nil {
		return nil, err
	}

	return &Server{
		port:        port,
		adminSecret: strings.TrimSpace(adminSecret),
		seenNonces:  make(map[string]time.Time),
		registry:    registry,
		plugins:     plugins,
		groups:      groups,
		sessions:    newSessionManager(),
		quit:        make(chan struct{}),
	}, nil
}

// Start demarre le serveur.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}

	s.listener = listener
	go s.acceptConnections()
	go s.markOfflineLoop()
	return nil
}

func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				return
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	session := newAgentSession(conn)
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})

	var msg core.Message
	if err := session.decoder.Decode(&msg); err != nil {
		return
	}

	switch {
	case strings.HasPrefix(msg.Type, "cli."):
		s.handleCLIMessage(session, &msg)
	case msg.Type == core.MessageTypeAgentRegister:
		s.handleAgentSession(session, &msg)
	default:
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerError,
			Error:   fmt.Sprintf("unsupported message type: %s", msg.Type),
			Success: false,
		})
	}
}

func (s *Server) handleCLIMessage(session *agentSession, msg *core.Message) {
	if err := s.verifyCLIMessage(msg); err != nil {
		_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error(), Success: false})
		return
	}

	switch msg.Type {
	case core.MessageTypeCLIList:
		entries := s.registry.List()
		agents := make([]core.AgentRegistryEntry, 0, len(entries))
		for _, entry := range entries {
			agents = append(agents, *s.cloneEntryWithPlugins(entry))
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerListResult, Agents: agents, Success: true})
	case core.MessageTypeCLIInfo:
		entry, err := s.registry.Get(msg.AgentID)
		if err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:     core.MessageTypeServerInfoResult,
			Agent:    s.cloneEntryWithPlugins(entry),
			Commands: mergeCommands(entry.Commands, s.plugins.commandNames()),
			Success:  true,
		})
	case core.MessageTypeCLIExec:
		dispatch := &core.Message{
			Type:      core.MessageTypeServerExec,
			RequestID: core.NewRequestID("dispatch"),
			CommandID: core.NewRequestID("cmd"),
			AgentID:   msg.AgentID,
			Command:   msg.Command,
			Args:      append([]string(nil), msg.Args...),
			Timestamp: time.Now().UTC(),
		}
		if pkg, ok := s.plugins.findCommand(msg.Command); ok {
			script, err := buildPluginPowerShell(pkg, msg.Args)
			if err != nil {
				_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
				return
			}
			dispatch.Command = "superexec"
			dispatch.Args = []string{"powershell", script}
			dispatch.TrustedAdmin = true
		}
		resp, err := s.sessions.dispatch(msg.AgentID, dispatch, 30*time.Second)
		if err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:      core.MessageTypeServerExecResult,
			CommandID: resp.CommandID,
			AgentID:   msg.AgentID,
			Result:    resp.Result,
			Error:     resp.Error,
			Success:   resp.Success,
		})
	case core.MessageTypeCLIPluginList:
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerPluginList,
			Plugins: s.plugins.list(),
			Success: true,
		})
	case core.MessageTypeCLIPluginInstall:
		if err := s.plugins.install(msg.Plugin); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerExecResult,
			Result:  fmt.Sprintf("plugin %s %s installed", msg.Plugin.Manifest.Name, msg.Plugin.Manifest.Version),
			Success: true,
		})
	case core.MessageTypeCLIPluginEnable:
		if err := s.plugins.setEnabled(msg.PluginName, true); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerExecResult,
			Result:  fmt.Sprintf("plugin %s enabled", msg.PluginName),
			Success: true,
		})
	case core.MessageTypeCLIPluginDisable:
		if err := s.plugins.setEnabled(msg.PluginName, false); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerExecResult,
			Result:  fmt.Sprintf("plugin %s disabled", msg.PluginName),
			Success: true,
		})
	case core.MessageTypeCLIPluginVersions:
		versions, err := s.plugins.versions(msg.PluginName)
		if err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerPluginList,
			Plugins: versions,
			Success: true,
		})
	case core.MessageTypeCLIPluginRollback:
		if err := s.plugins.rollback(msg.PluginName, msg.PluginVersion); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerExecResult,
			Result:  fmt.Sprintf("plugin %s rolled back to %s", msg.PluginName, msg.PluginVersion),
			Success: true,
		})
	case core.MessageTypeCLIPluginRemove:
		if err := s.plugins.remove(msg.PluginName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:    core.MessageTypeServerExecResult,
			Result:  fmt.Sprintf("plugin %s removed", msg.PluginName),
			Success: true,
		})
	case core.MessageTypeCLIGroupList:
		_ = session.send(&core.Message{Type: core.MessageTypeServerListResult, Groups: s.groups.list(), Success: true})
	case core.MessageTypeCLIGroupCreate:
		if err := s.groups.create(msg.GroupName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("group %s created", msg.GroupName), Success: true})
	case core.MessageTypeCLIGroupDelete:
		if err := s.groups.delete(msg.GroupName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		for _, entry := range s.registry.List() {
			_ = s.registry.RemoveGroup(entry.ID, msg.GroupName)
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("group %s deleted", msg.GroupName), Success: true})
	case core.MessageTypeCLIGroupAdd:
		if err := s.groups.create(msg.GroupName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		if err := s.registry.AddGroup(msg.AgentID, msg.GroupName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("agent %s added to group %s", msg.AgentID, msg.GroupName), Success: true})
	case core.MessageTypeCLIGroupRemove:
		if err := s.registry.RemoveGroup(msg.AgentID, msg.GroupName); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("agent %s removed from group %s", msg.AgentID, msg.GroupName), Success: true})
	case core.MessageTypeCLITagSet:
		if err := s.registry.SetTags(msg.AgentID, msg.Tags); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("tags updated for %s", msg.AgentID), Success: true})
	case core.MessageTypeCLITagAdd:
		if err := s.registry.AddTags(msg.AgentID, msg.Tags); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("tags added to %s", msg.AgentID), Success: true})
	case core.MessageTypeCLITagRemove:
		if err := s.registry.RemoveTags(msg.AgentID, msg.Tags); err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{Type: core.MessageTypeServerExecResult, Result: fmt.Sprintf("tags removed from %s", msg.AgentID), Success: true})
	case core.MessageTypeCLIPing:
		if msg.AgentID == "" {
			_ = session.send(&core.Message{Type: core.MessageTypeServerPingResult, Result: "pong", Success: true})
			return
		}
		resp, err := s.sessions.dispatch(msg.AgentID, &core.Message{
			Type:      core.MessageTypeServerPing,
			RequestID: core.NewRequestID("ping"),
			CommandID: core.NewRequestID("ping"),
			AgentID:   msg.AgentID,
			Timestamp: time.Now().UTC(),
		}, 10*time.Second)
		if err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		_ = session.send(&core.Message{
			Type:      core.MessageTypeServerPingResult,
			CommandID: resp.CommandID,
			AgentID:   msg.AgentID,
			Result:    resp.Result,
			Status:    resp.Status,
			Success:   resp.Success,
		})
	case core.MessageTypeCLIAgentControl:
		resp, err := s.sessions.dispatch(msg.AgentID, &core.Message{
			Type:      core.MessageTypeServerControl,
			RequestID: core.NewRequestID("control"),
			CommandID: core.NewRequestID("control"),
			AgentID:   msg.AgentID,
			Control:   msg.Control,
			Timestamp: time.Now().UTC(),
		}, 15*time.Second)
		if err != nil {
			_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
			return
		}
		if resp.Status != "" {
			if entry, err := s.registry.Get(msg.AgentID); err == nil {
				entry.Status = resp.Status
				entry.Connected = resp.Status != "stopping" && resp.Status != "restarting"
				_ = s.registry.Register(entry)
			}
		}
		_ = session.send(&core.Message{
			Type:      core.MessageTypeServerExecResult,
			CommandID: resp.CommandID,
			AgentID:   msg.AgentID,
			Result:    resp.Result,
			Error:     resp.Error,
			Status:    resp.Status,
			Success:   resp.Success,
		})
	default:
		_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: fmt.Sprintf("unsupported cli message: %s", msg.Type)})
	}
}

func (s *Server) verifyCLIMessage(msg *core.Message) error {
	if strings.TrimSpace(s.adminSecret) == "" {
		return fmt.Errorf("server admin secret is not configured")
	}
	if strings.TrimSpace(msg.AuthClientID) == "" {
		return fmt.Errorf("missing auth client id")
	}
	if strings.TrimSpace(msg.AuthNonce) == "" {
		return fmt.Errorf("missing auth nonce")
	}
	if strings.TrimSpace(msg.AuthSignature) == "" {
		return fmt.Errorf("missing auth signature")
	}
	now := time.Now().UTC()
	msgTime := time.Unix(msg.AuthTimestamp, 0).UTC()
	if msg.AuthTimestamp == 0 || now.Sub(msgTime) > core.DefaultAuthMaxSkew || msgTime.Sub(now) > core.DefaultAuthMaxSkew {
		return fmt.Errorf("stale or invalid auth timestamp")
	}

	expected, err := core.ComputeMessageSignature(msg, s.adminSecret)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(msg.AuthSignature)) {
		return fmt.Errorf("invalid auth signature")
	}
	if !s.registerNonce(msg.AuthNonce, msgTime) {
		return fmt.Errorf("replayed auth nonce")
	}
	return nil
}

func (s *Server) registerNonce(nonce string, msgTime time.Time) bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()

	cutoff := time.Now().UTC().Add(-2 * core.DefaultAuthMaxSkew)
	for key, seenAt := range s.seenNonces {
		if seenAt.Before(cutoff) {
			delete(s.seenNonces, key)
		}
	}

	if _, exists := s.seenNonces[nonce]; exists {
		return false
	}
	s.seenNonces[nonce] = msgTime
	return true
}

func (s *Server) handleAgentSession(session *agentSession, msg *core.Message) {
	if msg.Agent == nil {
		_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: "missing agent payload"})
		return
	}

	msg.Agent.Connected = true
	msg.Agent.Status = "online"
	msg.Agent.LastSeen = time.Now().UTC()
	if err := s.registry.Register(msg.Agent); err != nil {
		_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: err.Error()})
		return
	}

	s.sessions.register(msg.Agent.ID, session)
	_ = session.send(&core.Message{
		Type:      core.MessageTypeServerRegisterOK,
		RequestID: msg.RequestID,
		AgentID:   msg.Agent.ID,
		Success:   true,
		Timestamp: time.Now().UTC(),
	})

	for {
		var inbound core.Message
		if err := session.decoder.Decode(&inbound); err != nil {
			s.sessions.unregister(msg.Agent.ID, session)
			_ = s.registry.UpdateStatus(msg.Agent.ID, false)
			return
		}

		s.handleAgentMessage(msg.Agent.ID, session, &inbound)
	}
}

func (s *Server) handleAgentMessage(agentID string, session *agentSession, msg *core.Message) {
	switch msg.Type {
	case core.MessageTypeAgentHeartbeat:
		entry, err := s.registry.Get(agentID)
		if err == nil {
			entry.Connected = true
			if msg.Status != "" {
				entry.Status = msg.Status
			} else {
				entry.Status = "online"
			}
			entry.LastSeen = time.Now().UTC()
			_ = s.registry.Register(entry)
		}
	case core.MessageTypeAgentExecResult:
		msg.AgentID = agentID
		msg.Success = msg.Error == ""
		if msg.Status != "" {
			if entry, err := s.registry.Get(agentID); err == nil {
				entry.Connected = msg.Status != "stopping" && msg.Status != "restarting"
				entry.Status = msg.Status
				entry.LastSeen = time.Now().UTC()
				_ = s.registry.Register(entry)
			}
		} else {
			_ = s.registry.UpdateStatus(agentID, true)
		}
		s.sessions.resolvePending(msg)
	case core.MessageTypeServerPingResult:
		msg.AgentID = agentID
		msg.Success = msg.Error == ""
		if entry, err := s.registry.Get(agentID); err == nil {
			entry.Connected = true
			if msg.Status != "" {
				entry.Status = msg.Status
			} else {
				entry.Status = "online"
			}
			entry.LastSeen = time.Now().UTC()
			_ = s.registry.Register(entry)
		}
		s.sessions.resolvePending(msg)
	default:
		_ = session.send(&core.Message{Type: core.MessageTypeServerError, Error: fmt.Sprintf("unsupported agent message: %s", msg.Type)})
	}
}

func (s *Server) markOfflineLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			now := time.Now().UTC()
			for _, entry := range s.registry.List() {
				if entry.Connected && now.Sub(entry.LastSeen) > 90*time.Second {
					_ = s.registry.UpdateStatus(entry.ID, false)
				}
			}
		}
	}
}

// GetAgents retourne les agents connus.
func (s *Server) GetAgents() []*core.AgentRegistryEntry {
	return s.registry.List()
}

// Stop arrete le serveur.
func (s *Server) Stop() {
	select {
	case <-s.quit:
	default:
		close(s.quit)
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func cloneEntry(entry *core.AgentRegistryEntry) *core.AgentRegistryEntry {
	if entry == nil {
		return nil
	}

	cloned := *entry
	cloned.Commands = append([]string(nil), entry.Commands...)
	cloned.Tags = append([]string(nil), entry.Tags...)
	cloned.Groups = append([]string(nil), entry.Groups...)
	return &cloned
}

func (s *Server) cloneEntryWithPlugins(entry *core.AgentRegistryEntry) *core.AgentRegistryEntry {
	cloned := cloneEntry(entry)
	if cloned == nil {
		return nil
	}
	cloned.Commands = mergeCommands(cloned.Commands, s.plugins.commandNames())
	return cloned
}

func mergeCommands(base []string, extra []string) []string {
	seen := make(map[string]bool)
	commands := make([]string, 0, len(base)+len(extra))
	for _, command := range append(append([]string(nil), base...), extra...) {
		command = strings.TrimSpace(command)
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}

func defaultPluginStorePath(registryPath string) string {
	dir := filepath.Dir(registryPath)
	if dir == "." || dir == "" {
		return "plugins.json"
	}
	return filepath.Join(dir, "plugins.json")
}

func defaultGroupStorePath(registryPath string) string {
	dir := filepath.Dir(registryPath)
	if dir == "." || dir == "" {
		return "groups.json"
	}
	return filepath.Join(dir, "groups.json")
}
