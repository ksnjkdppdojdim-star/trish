//go:build windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type InstallOptions struct {
	ServerAddr string
	ServerPort int
	ListenPort int
	DryRun     bool
	NoStart    bool
}

// InstallOrRepair installe ou repare l'agent Windows.
func InstallOrRepair(opts InstallOptions, stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}

	cfg := &Config{
		ServerAddr: opts.ServerAddr,
		ServerPort: opts.ServerPort,
		ListenPort: opts.ListenPort,
		InstallDir: DefaultInstallDir(),
		LogDir:     DefaultLogDir(),
		Version:    agentVersion,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if opts.DryRun {
		return InstallCheck(opts, stdout)
	}

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	stdout("Preparation du dossier d'installation: %s", cfg.InstallDir)
	if err := os.MkdirAll(cfg.InstallDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	targetExe := DefaultExecutablePath()

	stdout("Arret preventif de l'ancien service si present...")
	if err := stopServiceIfPresent(); err != nil {
		return err
	}

	stdout("Copie du binaire dans %s", targetExe)
	if err := copyFile(currentExe, targetExe); err != nil {
		return err
	}

	stdout("Ecriture de la configuration: %s", DefaultConfigPath())
	if err := cfg.Save(DefaultConfigPath()); err != nil {
		return err
	}

	if serviceExists() {
		stdout("Service deja present: mise a jour de la configuration du service")
		if err := runVisible("sc.exe", "config", ServiceName, "binPath=", fmt.Sprintf("\"%s\" run-service", targetExe), "start=", "auto", "DisplayName=", ServiceDisplayName); err != nil {
			return err
		}
	} else {
		stdout("Creation du service Windows %s", ServiceName)
		if err := runVisible("sc.exe", "create", ServiceName, "binPath=", fmt.Sprintf("\"%s\" run-service", targetExe), "start=", "auto", "DisplayName=", ServiceDisplayName); err != nil {
			return err
		}
	}

	stdout("Configuration du redemarrage automatique du service")
	_ = runVisible("sc.exe", "failure", ServiceName, "reset=", "86400", "actions=", "restart/5000/restart/5000/restart/5000")

	if !opts.NoStart {
		stdout("Demarrage du service")
		if err := startServiceAndWait(); err != nil {
			return err
		}
	}

	stdout("Verification de la configuration installee")
	installedCfg, err := LoadConfig(DefaultConfigPath())
	if err != nil {
		return err
	}
	if err := installedCfg.Validate(); err != nil {
		return err
	}

	if !opts.NoStart {
		stdout("Verification de l'etat du service")
		state, err := queryServiceState()
		if err != nil {
			return err
		}
		if state != "RUNNING" {
			return fmt.Errorf("service %s not running after installation (state=%s)", ServiceName, state)
		}
	}

	stdout("Installation terminee. Le service tourne maintenant en arriere-plan.")
	return nil
}

// InstallCheck valide les prerequis et l'etat du terrain sans modifier le systeme.
func InstallCheck(opts InstallOptions, stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}

	cfg := &Config{
		ServerAddr: opts.ServerAddr,
		ServerPort: opts.ServerPort,
		ListenPort: opts.ListenPort,
		InstallDir: DefaultInstallDir(),
		LogDir:     DefaultLogDir(),
		Version:    agentVersion,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	stdout("Verification de la configuration cible")
	stdout("Serveur cible: %s:%d", cfg.ServerAddr, cfg.ServerPort)
	stdout("Dossier d'installation: %s", cfg.InstallDir)
	stdout("Fichier de configuration: %s", DefaultConfigPath())
	stdout("Fichier binaire cible: %s", DefaultExecutablePath())

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	stdout("Binaire courant: %s", currentExe)

	if serviceExists() {
		state, err := queryServiceState()
		if err != nil {
			return err
		}
		stdout("Service existant detecte: %s (state=%s)", ServiceName, state)
	} else {
		stdout("Aucun service existant detecte")
	}

	if _, err := os.Stat(DefaultConfigPath()); err == nil {
		installedCfg, err := LoadConfig(DefaultConfigPath())
		if err != nil {
			return err
		}
		stdout("Configuration existante detectee pour %s:%d", installedCfg.ServerAddr, installedCfg.ServerPort)
	} else {
		stdout("Aucune configuration existante detectee")
	}

	stdout("Install-check termine. Aucun changement n'a ete applique.")
	return nil
}

// Uninstall supprime le service et les fichiers connus.
func Uninstall(stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	stdout("Arret du service")
	_ = runVisible("sc.exe", "stop", ServiceName)

	if serviceExists() {
		stdout("Suppression du service")
		if err := runVisible("sc.exe", "delete", ServiceName); err != nil {
			return err
		}
	}

	stdout("Suppression de la configuration et des binaires installes")
	_ = os.Remove(DefaultConfigPath())
	_ = os.Remove(DefaultExecutablePath())
	_ = os.RemoveAll(DefaultLogDir())
	_ = os.RemoveAll(filepath.Dir(DefaultConfigPath()))

	stdout("Desinstallation terminee")
	return nil
}

func ensureAdmin() error {
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		return ErrAdminRequired
	}
	return nil
}

func serviceExists() bool {
	return exec.Command("sc.exe", "query", ServiceName).Run() == nil
}

func queryServiceState() (string, error) {
	cmd := exec.Command("sc.exe", "query", ServiceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sc.exe query failed: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STATE") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return parts[3], nil
			}
		}
	}
	return "", fmt.Errorf("unable to parse service state")
}

func waitForServiceState(expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := queryServiceState()
		if err == nil && state == expected {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	state, err := queryServiceState()
	if err != nil {
		return err
	}
	return fmt.Errorf("service %s did not reach state %s (current=%s)", ServiceName, expected, state)
}

func stopServiceIfPresent() error {
	if !serviceExists() {
		return nil
	}

	state, err := queryServiceState()
	if err == nil && state == "STOPPED" {
		return nil
	}

	_ = runHidden("sc.exe", "stop", ServiceName)
	return waitForServiceState("STOPPED", 20*time.Second)
}

func startServiceAndWait() error {
	if err := runVisible("sc.exe", "start", ServiceName); err != nil && !strings.Contains(strings.ToLower(err.Error()), "service has already been started") {
		return err
	}
	return waitForServiceState("RUNNING", 20*time.Second)
}

func runVisible(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %v (%s)", name, args, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Close()
}
