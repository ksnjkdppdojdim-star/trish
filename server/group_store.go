package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type groupStore struct {
	mu     sync.RWMutex
	path   string
	groups map[string]bool
}

func newGroupStore(path string) (*groupStore, error) {
	store := &groupStore{
		path:   path,
		groups: make(map[string]bool),
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return store, nil
}

func (s *groupStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var groups []string
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group != "" {
			s.groups[group] = true
		}
	}
	return nil
}

func (s *groupStore) create(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	s.mu.Lock()
	s.groups[name] = true
	groups := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(groups)
}

func (s *groupStore) delete(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	s.mu.Lock()
	if !s.groups[name] {
		s.mu.Unlock()
		return fmt.Errorf("group %s not found", name)
	}
	delete(s.groups, name)
	groups := s.snapshotLocked()
	s.mu.Unlock()
	return s.save(groups)
}

func (s *groupStore) list() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups := make([]string, 0, len(s.groups))
	for group := range s.groups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func (s *groupStore) snapshotLocked() []string {
	groups := make([]string, 0, len(s.groups))
	for group := range s.groups {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func (s *groupStore) save(groups []string) error {
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
