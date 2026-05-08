// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package dir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"trish/core"
)

// DirCommand implemente la commande dir.
type DirCommand struct {
	state *core.SessionState
}

// NewDirCommand cree une nouvelle instance.
func NewDirCommand(state *core.SessionState) *DirCommand {
	if state == nil {
		state = core.NewSessionState()
	}

	return &DirCommand{state: state}
}

func (dc *DirCommand) Name() string {
	return "dir"
}

func (dc *DirCommand) Description() string {
	return "List directory contents"
}

func (dc *DirCommand) Execute(args []string) (string, error) {
	targetDir := dc.state.CurrentDir()
	if len(args) > 0 {
		if filepath.IsAbs(args[0]) {
			targetDir = args[0]
		} else {
			targetDir = filepath.Join(dc.state.CurrentDir(), args[0])
		}
	}

	files, err := os.ReadDir(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory of %s\n\n", targetDir))

	var totalSize int64
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.IsDir() {
			result.WriteString(fmt.Sprintf("%-20s <DIR>       %s\n",
				info.ModTime().Format("01/02/2006 15:04"),
				file.Name()))
			continue
		}

		totalSize += info.Size()
		result.WriteString(fmt.Sprintf("%-20s %12d  %s\n",
			info.ModTime().Format("01/02/2006 15:04"),
			info.Size(),
			file.Name()))
	}

	result.WriteString(fmt.Sprintf("\nTotal: %d files, %d bytes\n", len(files), totalSize))
	return result.String(), nil
}
