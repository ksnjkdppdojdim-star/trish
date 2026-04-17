package dir

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// DirCommand implémente la commande dir
type DirCommand struct {
	currentDir string
}

// NewDirCommand crée une nouvelle instance
func NewDirCommand() *DirCommand {
	cwd, _ := os.Getwd()
	return &DirCommand{
		currentDir: cwd,
	}
}

func (dc *DirCommand) Name() string {
	return "dir"
}

func (dc *DirCommand) Description() string {
	return "List directory contents"
}

func (dc *DirCommand) Execute(args []string) (string, error) {
	targetDir := dc.currentDir

	if len(args) > 0 {
		// Si un chemin est fourni, utiliser celui-ci
		if filepath.IsAbs(args[0]) {
			targetDir = args[0]
		} else {
			targetDir = filepath.Join(dc.currentDir, args[0])
		}
	}

	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory of %s\n\n", targetDir))

	var totalSize int64
	for _, file := range files {
		mode := file.Mode()
		if mode.IsDir() {
			result.WriteString(fmt.Sprintf("%-20s <DIR>       %s\n",
				file.ModTime().Format("01/02/2006 15:04"),
				file.Name()))
		} else {
			totalSize += file.Size()
			result.WriteString(fmt.Sprintf("%-20s %12d  %s\n",
				file.ModTime().Format("01/02/2006 15:04"),
				file.Size(),
				file.Name()))
		}
	}

	result.WriteString(fmt.Sprintf("\nTotal: %d files, %d bytes\n", len(files), totalSize))
	return result.String(), nil
}
