package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"trish/core"
)

func TestVerifyCLIMessageAcceptsSignedMessage(t *testing.T) {
	registryPath := filepath.Join("..", ".testdata", "server-auth-accepts.json")
	_ = os.MkdirAll(filepath.Dir(registryPath), 0700)
	_ = os.Remove(registryPath)
	srv, err := NewServer(9999, registryPath, "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := &core.Message{
		Type:          core.MessageTypeCLIList,
		RequestID:     "cli-1",
		AuthClientID:  "client-a",
		AuthTimestamp: time.Now().UTC().Unix(),
		AuthNonce:     "nonce-1",
	}
	if err := core.SignMessage(msg, "secret"); err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}

	if err := srv.verifyCLIMessage(msg); err != nil {
		t.Fatalf("expected valid auth, got %v", err)
	}
}

func TestVerifyCLIMessageRejectsReplay(t *testing.T) {
	registryPath := filepath.Join("..", ".testdata", "server-auth-replay.json")
	_ = os.MkdirAll(filepath.Dir(registryPath), 0700)
	_ = os.Remove(registryPath)
	srv, err := NewServer(9999, registryPath, "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := &core.Message{
		Type:          core.MessageTypeCLIList,
		RequestID:     "cli-1",
		AuthClientID:  "client-a",
		AuthTimestamp: time.Now().UTC().Unix(),
		AuthNonce:     "nonce-1",
	}
	if err := core.SignMessage(msg, "secret"); err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}

	if err := srv.verifyCLIMessage(msg); err != nil {
		t.Fatalf("expected first auth to pass, got %v", err)
	}
	if err := srv.verifyCLIMessage(msg); err == nil {
		t.Fatalf("expected replay to fail")
	}
}
