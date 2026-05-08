// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package server

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessLock empeche plusieurs serveurs sur le meme host.
type ProcessLock struct {
	file *os.File
	path string
}

// AcquireProcessLock prend un verrou exclusif base sur un fichier.
func AcquireProcessLock(path string) (*ProcessLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			stale, staleErr := removeIfStale(path)
			if staleErr != nil {
				return nil, staleErr
			}
			if stale {
				return AcquireProcessLock(path)
			}
			return nil, fmt.Errorf("server already running (lock: %s)", path)
		}
		return nil, err
	}

	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}

	return &ProcessLock{file: file, path: path}, nil
}

// Release libere le verrou.
func (l *ProcessLock) Release() {
	if l == nil {
		return
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	if l.path != "" {
		_ = os.Remove(l.path)
	}
}

func removeIfStale(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return false, removeErr
		}
		return true, nil
	}

	running, err := processRunning(pid)
	if err != nil {
		return false, err
	}
	if running {
		return false, nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	return true, nil
}

func processRunning(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	line := strings.TrimSpace(string(bytes.TrimSpace(output)))
	if line == "" {
		return false, nil
	}
	if strings.Contains(strings.ToLower(line), "no tasks are running") {
		return false, nil
	}
	return strings.Contains(line, fmt.Sprintf("\"%d\"", pid)), nil
}
