// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

//go:build windows

package agent

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

var ErrAdminRequired = errors.New("administrator privileges are required to install the Trish agent")

var (
	modShell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = modShell32.NewProc("ShellExecuteW")
)

// IsAdminRequired retourne vrai si l'erreur demande une elevation admin.
func IsAdminRequired(err error) bool {
	return errors.Is(err, ErrAdminRequired)
}

// RelaunchElevated relance le binaire courant avec UAC.
func RelaunchElevated(args []string) error {
	exe, err := syscall.UTF16PtrFromString(currentExecutable())
	if err != nil {
		return err
	}

	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}

	params, err := syscall.UTF16PtrFromString(joinWindowsArgs(args))
	if err != nil {
		return err
	}

	ret, _, callErr := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exe)),
		uintptr(unsafe.Pointer(params)),
		0,
		1,
	)
	if ret <= 32 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return ErrAdminRequired
	}
	return nil
}

func joinWindowsArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, `""`)
			continue
		}
		if strings.ContainsAny(arg, " \t\"") {
			arg = strings.ReplaceAll(arg, `"`, `\"`)
			quoted = append(quoted, `"`+arg+`"`)
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
