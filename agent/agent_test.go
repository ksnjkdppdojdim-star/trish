// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package agent

import (
	"strings"
	"testing"
	"trish/core"
)

func TestSuperExecRequiresTrustedAdmin(t *testing.T) {
	svc := NewAgentService("127.0.0.1", 9999, 2222)

	_, err := svc.executeSuperExec(&core.Message{
		Command:      "superexec",
		Args:         []string{"echo", "hello"},
		TrustedAdmin: false,
	})
	if err == nil || !strings.Contains(err.Error(), "trusted admin") {
		t.Fatalf("expected trusted admin error, got %v", err)
	}
}

func TestSuperExecRequiresCommand(t *testing.T) {
	svc := NewAgentService("127.0.0.1", 9999, 2222)

	_, err := svc.executeSuperExec(&core.Message{
		Command:      "superexec",
		TrustedAdmin: true,
	})
	if err == nil || !strings.Contains(err.Error(), "usage: superexec") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestSuperExecExecModeRequiresProgram(t *testing.T) {
	svc := NewAgentService("127.0.0.1", 9999, 2222)

	_, err := svc.executeSuperExec(&core.Message{
		Command:      "superexec",
		Args:         []string{"exec"},
		TrustedAdmin: true,
	})
	if err == nil || !strings.Contains(err.Error(), "usage: superexec exec") {
		t.Fatalf("expected exec usage error, got %v", err)
	}
}
