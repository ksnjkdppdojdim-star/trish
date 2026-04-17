package core

import (
	"sync"
	"time"
)

// EventType définit les types d'événements possibles
type EventType string

const (
	EventCommandExecuted  EventType = "command_executed"
	EventAgentConnected   EventType = "agent_connected"
	EventAgentDisconnected EventType = "agent_disconnected"
	EventError            EventType = "error"
)

// Event représente un événement dans le système
type Event struct {
	Type      EventType
	Timestamp time.Time
	Source    string // Agent ID ou Client ID
	Message   string
	Data      map[string]interface{}
}

// EventBus gère les événements du système
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]chan *Event
}

// NewEventBus crée un nouveau bus événements
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]chan *Event),
	}
}

// Subscribe s'abonne à un type d'événement
func (eb *EventBus) Subscribe(eventType EventType) <-chan *Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan *Event, 10)
	key := string(eventType)
	eb.listeners[key] = append(eb.listeners[key], ch)
	return ch
}

// Publish publie un événement
func (eb *EventBus) Publish(event *Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	key := string(event.Type)
	for _, ch := range eb.listeners[key] {
		select {
		case ch <- event:
		default:
			// Channel plein, skip
		}
	}
}

// UnsubscribeAll ferme tous les listeners
func (eb *EventBus) UnsubscribeAll() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for _, listeners := range eb.listeners {
		for _, ch := range listeners {
			close(ch)
		}
	}
	eb.listeners = make(map[string][]chan *Event)
}
