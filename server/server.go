package server

import (
	"crypto/hmac"
	"fmt"
	"net"
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

	return &Server{
		port:        port,
		adminSecret: strings.TrimSpace(adminSecret),
		seenNonces:  make(map[string]time.Time),
		registry:    registry,
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
			agents = append(agents, *cloneEntry(entry))
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
			Agent:    cloneEntry(entry),
			Commands: append([]string(nil), entry.Commands...),
			Success:  true,
		})
	case core.MessageTypeCLIExec:
		resp, err := s.sessions.dispatch(msg.AgentID, &core.Message{
			Type:         core.MessageTypeServerExec,
			RequestID:    core.NewRequestID("dispatch"),
			CommandID:    core.NewRequestID("cmd"),
			AgentID:      msg.AgentID,
			Command:      msg.Command,
			Args:         append([]string(nil), msg.Args...),
			TrustedAdmin: true,
			Timestamp:    time.Now().UTC(),
		}, 30*time.Second)
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
	return &cloned
}
