package cd

import (
	"fmt"
	"os"
	"path/filepath"
)

// CdCommand implémente la commande cd
type CdCommand struct {
	currentDir string
}

// NewCdCommand crée une nouvelle instance
func NewCdCommand() *CdCommand {
	cwd, _ := os.Getwd()
	return &CdCommand{
		currentDir: cwd,
	}
}

func (cc *CdCommand) Name() string {
	return "cd"
}

func (cc *CdCommand) Description() string {
	return "Change directory"
}

func (cc *CdCommand) Execute(args []string) (string, error) {
	if len(args) == 0 {
		return cc.currentDir, nil
	}

	targetDir := args[0]

	// Gérer les chemins relatifs
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(cc.currentDir, targetDir)
	}

	// Normaliser le chemin
	targetDir = filepath.Clean(targetDir)

	// Vérifier que le répertoire existe
	info, err := os.Stat(targetDir)
	if err != nil {
		return "", fmt.Errorf("directory not found: %s", targetDir)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", targetDir)
	}

	cc.currentDir = targetDir
	return fmt.Sprintf("Changed directory to: %s", cc.currentDir), nil
}

// GetCurrentDir retourne le répertoire courant
func (cc *CdCommand) GetCurrentDir() string {
	return cc.currentDir
}
