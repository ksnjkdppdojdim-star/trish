package agent

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"
	"trish/core"
)

var agentVersion = core.Version

// AgentService represente le service agent qui tourne sur une machine distante.
type AgentService struct {
	agent      *core.Agent
	serverAddr string
	serverPort int
	listenPort int
	frozen     bool

	mu      sync.RWMutex
	conn    net.Conn
	decoder *core.Decoder
	encoder *core.Encoder
	writeMu sync.Mutex
	quit    chan struct{}
}

// NewAgentService cree un nouveau service agent.
func NewAgentService(serverAddr string, serverPort int, listenPort int) *AgentService {
	hostname, _ := os.Hostname()
	agent := core.NewAgent(hostname)
	_ = agent.Start()

	if serverAddr == "" {
		serverAddr = "127.0.0.1"
	}
	if serverPort == 0 {
		serverPort = 9999
	}
	if listenPort == 0 {
		listenPort = 2222
	}

	agent.Port = listenPort

	return &AgentService{
		agent:      agent,
		serverAddr: serverAddr,
		serverPort: serverPort,
		listenPort: listenPort,
		quit:       make(chan struct{}),
	}
}

// Agent retourne l'agent courant.
func (as *AgentService) Agent() *core.Agent {
	return as.agent
}

// Start demarre le service agent.
func (as *AgentService) Start() error {
	go as.connectionLoop()
	return nil
}

func (as *AgentService) connectionLoop() {
	backoff := 2 * time.Second

	for {
		select {
		case <-as.quit:
			return
		default:
		}

		if err := as.connectAndServe(); err != nil {
			time.Sleep(backoff)
			if backoff < 15*time.Second {
				backoff += 2 * time.Second
			}
			continue
		}

		backoff = 2 * time.Second
	}
}

func (as *AgentService) connectAndServe() error {
	addr := fmt.Sprintf("%s:%d", as.serverAddr, as.serverPort)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}

	decoder := core.NewDecoder(conn)
	encoder := core.NewEncoder(conn)

	as.mu.Lock()
	as.conn = conn
	as.decoder = decoder
	as.encoder = encoder
	as.mu.Unlock()

	register := &core.Message{
		Type:      core.MessageTypeAgentRegister,
		RequestID: core.NewRequestID("register"),
		AgentID:   as.agent.ID,
		Agent: &core.AgentRegistryEntry{
			ID:        as.agent.ID,
			MachineID: as.machineID(),
			Hostname:  as.agent.Hostname,
			IPAddress: as.advertisedIP(),
			Port:      as.agent.Port,
			Version:   agentVersion,
			Status:    "online",
			Connected: true,
			Commands:  as.agent.Registry.List(),
		},
		Timestamp: time.Now().UTC(),
	}

	if err := encoder.Encode(register); err != nil {
		as.clearConnection()
		_ = conn.Close()
		return err
	}

	var ack core.Message
	if err := decoder.Decode(&ack); err != nil {
		as.clearConnection()
		_ = conn.Close()
		return err
	}
	if ack.Type != core.MessageTypeServerRegisterOK {
		as.clearConnection()
		_ = conn.Close()
		if ack.Error != "" {
			return fmt.Errorf(ack.Error)
		}
		return fmt.Errorf("unexpected register response: %s", ack.Type)
	}

	done := make(chan struct{})
	go as.heartbeatLoop(done)

	for {
		var msg core.Message
		if err := decoder.Decode(&msg); err != nil {
			close(done)
			as.clearConnection()
			_ = conn.Close()
			return err
		}

		if err := as.handleServerMessage(&msg); err != nil {
			close(done)
			as.clearConnection()
			_ = conn.Close()
			return err
		}
	}
}

func (as *AgentService) heartbeatLoop(done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-as.quit:
			return
		case <-done:
			return
		case <-ticker.C:
			_ = as.send(&core.Message{
				Type:      core.MessageTypeAgentHeartbeat,
				RequestID: core.NewRequestID("hb"),
				AgentID:   as.agent.ID,
				Status:    as.currentStatus(),
				Timestamp: time.Now().UTC(),
			})
		}
	}
}

func (as *AgentService) handleServerMessage(msg *core.Message) error {
	switch msg.Type {
	case core.MessageTypeServerExec:
		if as.isFrozen() {
			return as.send(&core.Message{
				Type:      core.MessageTypeAgentExecResult,
				RequestID: msg.RequestID,
				CommandID: msg.CommandID,
				AgentID:   as.agent.ID,
				Error:     "agent is frozen and not accepting commands",
				Success:   false,
				Status:    "frozen",
				Timestamp: time.Now().UTC(),
			})
		}
		result, err := as.agent.ExecuteCommand(msg.Command, msg.Args)
		reply := &core.Message{
			Type:      core.MessageTypeAgentExecResult,
			RequestID: msg.RequestID,
			CommandID: msg.CommandID,
			AgentID:   as.agent.ID,
			Result:    result,
			Success:   err == nil,
			Timestamp: time.Now().UTC(),
		}
		if err != nil {
			reply.Error = err.Error()
		}
		return as.send(reply)
	case core.MessageTypeServerControl:
		return as.handleControlMessage(msg)
	case core.MessageTypeServerPing:
		return as.send(&core.Message{
			Type:      core.MessageTypeServerPingResult,
			RequestID: msg.RequestID,
			CommandID: msg.CommandID,
			AgentID:   as.agent.ID,
			Result:    "pong",
			Status:    as.currentStatus(),
			Success:   true,
			Timestamp: time.Now().UTC(),
		})
	case core.MessageTypeServerError:
		return fmt.Errorf(msg.Error)
	default:
		return fmt.Errorf("unsupported server message: %s", msg.Type)
	}
}

func (as *AgentService) handleControlMessage(msg *core.Message) error {
	reply := &core.Message{
		Type:      core.MessageTypeAgentExecResult,
		RequestID: msg.RequestID,
		CommandID: msg.CommandID,
		AgentID:   as.agent.ID,
		Success:   true,
		Timestamp: time.Now().UTC(),
	}

	switch msg.Control {
	case "freeze":
		as.setFrozen(true)
		reply.Result = "agent frozen"
		reply.Status = "frozen"
	case "unfreeze":
		as.setFrozen(false)
		reply.Result = "agent unfrozen"
		reply.Status = "online"
	case "stop":
		reply.Result = "agent stopping"
		reply.Status = "stopping"
		if err := as.send(reply); err != nil {
			return err
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}()
		return nil
	case "restart":
		reply.Result = "agent restarting"
		reply.Status = "restarting"
		if err := as.send(reply); err != nil {
			return err
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(1)
		}()
		return nil
	default:
		reply.Success = false
		reply.Error = fmt.Sprintf("unsupported agent control: %s", msg.Control)
	}

	return as.send(reply)
}

func (as *AgentService) send(msg *core.Message) error {
	as.mu.RLock()
	encoder := as.encoder
	as.mu.RUnlock()
	if encoder == nil {
		return fmt.Errorf("agent not connected")
	}
	as.writeMu.Lock()
	defer as.writeMu.Unlock()
	return encoder.Encode(msg)
}

func (as *AgentService) clearConnection() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.conn = nil
	as.decoder = nil
	as.encoder = nil
}

func (as *AgentService) setFrozen(value bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.frozen = value
}

func (as *AgentService) isFrozen() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.frozen
}

func (as *AgentService) currentStatus() string {
	if as.isFrozen() {
		return "frozen"
	}
	return "online"
}

func (as *AgentService) machineID() string {
	return as.agent.Hostname
}

func (as *AgentService) advertisedIP() string {
	if as.serverAddr == "localhost" || as.serverAddr == "127.0.0.1" {
		return "127.0.0.1"
	}
	if as.agent.IPAddress != "" {
		return as.agent.IPAddress
	}
	return "127.0.0.1"
}

// Stop arrete le service.
func (as *AgentService) Stop() {
	select {
	case <-as.quit:
	default:
		close(as.quit)
	}

	as.mu.RLock()
	conn := as.conn
	as.mu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// KeepAlive reste pour compatibilite, le heartbeat tourne deja sur la session.
func (as *AgentService) KeepAlive() {}
