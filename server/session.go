package server

import (
	"fmt"
	"net"
	"sync"
	"time"
	"trish/core"
)

type agentSession struct {
	id      string
	conn    net.Conn
	decoder *core.Decoder
	encoder *core.Encoder
	writeMu sync.Mutex
}

func newAgentSession(conn net.Conn) *agentSession {
	return &agentSession{
		conn:    conn,
		decoder: core.NewDecoder(conn),
		encoder: core.NewEncoder(conn),
	}
}

func (s *agentSession) send(msg *core.Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.encoder.Encode(msg)
}

type sessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*agentSession
	pending  map[string]chan *core.Message
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		sessions: make(map[string]*agentSession),
		pending:  make(map[string]chan *core.Message),
	}
}

func (m *sessionManager) register(agentID string, session *agentSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if previous, ok := m.sessions[agentID]; ok && previous != nil && previous.conn != nil && previous.conn != session.conn {
		_ = previous.conn.Close()
	}

	session.id = agentID
	m.sessions[agentID] = session
}

func (m *sessionManager) unregister(agentID string, session *agentSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.sessions[agentID]
	if ok && current == session {
		delete(m.sessions, agentID)
	}
}

func (m *sessionManager) get(agentID string) (*agentSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[agentID]
	return session, ok
}

func (m *sessionManager) registerPending(commandID string) chan *core.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *core.Message, 1)
	m.pending[commandID] = ch
	return ch
}

func (m *sessionManager) resolvePending(msg *core.Message) bool {
	m.mu.Lock()
	ch, ok := m.pending[msg.CommandID]
	if ok {
		delete(m.pending, msg.CommandID)
	}
	m.mu.Unlock()

	if ok {
		ch <- msg
		close(ch)
	}
	return ok
}

func (m *sessionManager) cancelPending(commandID string) {
	m.mu.Lock()
	ch, ok := m.pending[commandID]
	if ok {
		delete(m.pending, commandID)
	}
	m.mu.Unlock()

	if ok {
		close(ch)
	}
}

func (m *sessionManager) dispatch(agentID string, msg *core.Message, timeout time.Duration) (*core.Message, error) {
	session, ok := m.get(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s is offline", agentID)
	}

	wait := m.registerPending(msg.CommandID)
	if err := session.send(msg); err != nil {
		m.cancelPending(msg.CommandID)
		return nil, err
	}

	select {
	case resp, ok := <-wait:
		if !ok {
			return nil, fmt.Errorf("command %s cancelled", msg.CommandID)
		}
		return resp, nil
	case <-time.After(timeout):
		m.cancelPending(msg.CommandID)
		return nil, fmt.Errorf("command %s timed out", msg.CommandID)
	}
}
