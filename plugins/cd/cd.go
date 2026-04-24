package cd

import (
	"fmt"
	"os"
	"path/filepath"
	"trish/core"
)

// CdCommand implemente la commande cd.
type CdCommand struct {
	state *core.SessionState
}

// NewCdCommand cree une nouvelle instance.
func NewCdCommand(state *core.SessionState) *CdCommand {
	if state == nil {
		state = core.NewSessionState()
	}

	return &CdCommand{state: state}
}

func (cc *CdCommand) Name() string {
	return "cd"
}

func (cc *CdCommand) Description() string {
	return "Change directory"
}

func (cc *CdCommand) Execute(args []string) (string, error) {
	if len(args) == 0 {
		return cc.state.CurrentDir(), nil
	}

	targetDir := args[0]
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(cc.state.CurrentDir(), targetDir)
	}

	targetDir = filepath.Clean(targetDir)

	info, err := os.Stat(targetDir)
	if err != nil {
		return "", fmt.Errorf("directory not found: %s", targetDir)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", targetDir)
	}

	cc.state.SetCurrentDir(targetDir)
	return fmt.Sprintf("Changed directory to: %s", cc.state.CurrentDir()), nil
}

// GetCurrentDir retourne le repertoire courant.
func (cc *CdCommand) GetCurrentDir() string {
	return cc.state.CurrentDir()
}
