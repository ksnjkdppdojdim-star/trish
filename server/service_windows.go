//go:build windows

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	ServerServiceName        = "TrishServer"
	ServerServiceDisplayName = "Trish Server"

	serviceWin32OwnProcess = 0x00000010
	serviceStopPending     = 0x00000003
	serviceRunning         = 0x00000004
	serviceStartPending    = 0x00000002
	serviceStopped         = 0x00000001
	serviceAcceptStop      = 0x00000001
	serviceAcceptShutdown  = 0x00000004
	serviceControlStop     = 0x00000001
	serviceControlShutdown = 0x00000005
	serviceNoError         = 0x00000000
)

type ServiceInstallOptions struct {
	Port         int
	RegistryPath string
	LockPath     string
	AdminSecret  string
	ConfigData   []byte
	NoStart      bool
}

type serviceTableEntry struct {
	name *uint16
	proc uintptr
}

type serviceStatus struct {
	serviceType             uint32
	currentState            uint32
	controlsAccepted        uint32
	win32ExitCode           uint32
	serviceSpecificExitCode uint32
	checkPoint              uint32
	waitHint                uint32
}

type windowsService struct {
	statusHandle uintptr
	stopOnce     sync.Once
	stopCh       chan struct{}
	doneCh       chan struct{}
	runErr       error
}

var (
	modAdvapi32                    = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcher = modAdvapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandler = modAdvapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus           = modAdvapi32.NewProc("SetServiceStatus")

	currentService     *windowsService
	currentServiceOnce sync.Once
)

func DefaultInstallDir() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
	}
	return filepath.Join(programData, "Trish", "server")
}

func DefaultExecutablePath() string {
	return filepath.Join(DefaultInstallDir(), "trish-server.exe")
}

func DefaultConfigPath() string {
	return filepath.Join(DefaultInstallDir(), "server-config.json")
}

func SaveServiceConfig(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(DefaultConfigPath()), 0700); err != nil {
		return err
	}
	return os.WriteFile(DefaultConfigPath(), data, 0600)
}

func InstallOrRepairService(opts ServiceInstallOptions, stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}
	if err := validateInstallOptions(opts); err != nil {
		return err
	}

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	stdout("Preparation du dossier d'installation: %s", DefaultInstallDir())
	if err := os.MkdirAll(DefaultInstallDir(), 0755); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return err
	}

	stdout("Arret preventif de l'ancien service si present...")
	if err := stopServiceIfPresent(); err != nil {
		return err
	}

	stdout("Copie du binaire dans %s", DefaultExecutablePath())
	if err := copyFile(currentExe, DefaultExecutablePath()); err != nil {
		return err
	}

	stdout("Ecriture de la configuration: %s", DefaultConfigPath())
	if err := SaveServiceConfig(opts.ConfigData); err != nil {
		return err
	}

	if serviceExists() {
		stdout("Service deja present: mise a jour de la configuration du service")
		if err := runVisible("sc.exe", "config", ServerServiceName, "binPath=", fmt.Sprintf("\"%s\" run-service", DefaultExecutablePath()), "start=", "auto", "DisplayName=", ServerServiceDisplayName); err != nil {
			return err
		}
	} else {
		stdout("Creation du service Windows %s", ServerServiceName)
		if err := runVisible("sc.exe", "create", ServerServiceName, "binPath=", fmt.Sprintf("\"%s\" run-service", DefaultExecutablePath()), "start=", "auto", "DisplayName=", ServerServiceDisplayName); err != nil {
			return err
		}
	}

	stdout("Configuration du redemarrage automatique du service")
	_ = runVisible("sc.exe", "failure", ServerServiceName, "reset=", "86400", "actions=", "restart/5000/restart/5000/restart/5000")

	if !opts.NoStart {
		stdout("Demarrage du service")
		if err := startServiceAndWait(); err != nil {
			return err
		}
	}

	stdout("Installation terminee. Le service serveur tournera automatiquement au demarrage du PC.")
	return nil
}

func InstallCheckService(opts ServiceInstallOptions, stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}
	if err := validateInstallOptions(opts); err != nil {
		return err
	}

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	stdout("Binaire courant: %s", currentExe)
	stdout("Binaire cible: %s", DefaultExecutablePath())
	stdout("Fichier de configuration: %s", DefaultConfigPath())
	stdout("Port serveur: %d", opts.Port)
	stdout("Registre: %s", opts.RegistryPath)
	stdout("Lock: %s", opts.LockPath)

	if serviceExists() {
		state, err := queryServiceState()
		if err != nil {
			return err
		}
		stdout("Service existant detecte: %s (state=%s)", ServerServiceName, state)
	} else {
		stdout("Aucun service existant detecte")
	}

	stdout("Install-check termine. Aucun changement n'a ete applique.")
	return nil
}

func UninstallService(stdout func(string, ...interface{})) error {
	if stdout == nil {
		stdout = func(string, ...interface{}) {}
	}

	stdout("Verification des droits administrateur...")
	if err := ensureAdmin(); err != nil {
		return err
	}

	stdout("Arret du service")
	_ = runVisible("sc.exe", "stop", ServerServiceName)

	if serviceExists() {
		stdout("Suppression du service")
		if err := runVisible("sc.exe", "delete", ServerServiceName); err != nil {
			return err
		}
	}

	stdout("Suppression de la configuration et des binaires installes")
	_ = os.Remove(DefaultConfigPath())
	_ = os.Remove(DefaultExecutablePath())
	_ = os.RemoveAll(DefaultInstallDir())

	stdout("Desinstallation terminee")
	return nil
}

func RunServiceMode() error {
	currentServiceOnce = sync.Once{}
	currentService = &windowsService{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	namePtr, err := syscall.UTF16PtrFromString(ServerServiceName)
	if err != nil {
		return err
	}

	table := []serviceTableEntry{
		{name: namePtr, proc: syscall.NewCallback(serviceMain)},
		{},
	}

	r1, _, callErr := procStartServiceCtrlDispatcher.Call(uintptr(unsafe.Pointer(&table[0])))
	if r1 == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("failed to start service control dispatcher")
	}
	return currentService.runErr
}

func serviceMain(argc uint32, argv uintptr) uintptr {
	currentServiceOnce.Do(func() {
		namePtr, _ := syscall.UTF16PtrFromString(ServerServiceName)
		handle, _, _ := procRegisterServiceCtrlHandler.Call(
			uintptr(unsafe.Pointer(namePtr)),
			syscall.NewCallback(serviceControlHandler),
			0,
		)
		currentService.statusHandle = handle
		currentService.setStatus(serviceStartPending, 0)

		go func() {
			currentService.runErr = currentService.run()
			currentService.setStatus(serviceStopped, 0)
			close(currentService.doneCh)
		}()

		currentService.setStatus(serviceRunning, serviceAcceptStop|serviceAcceptShutdown)
	})

	<-currentService.doneCh
	return 0
}

func serviceControlHandler(control, eventType, eventData, context uintptr) uintptr {
	switch uint32(control) {
	case serviceControlStop, serviceControlShutdown:
		currentService.setStatus(serviceStopPending, 0)
		currentService.stop()
	}
	return 0
}

func (s *windowsService) setStatus(state uint32, accepted uint32) {
	status := serviceStatus{
		serviceType:      serviceWin32OwnProcess,
		currentState:     state,
		controlsAccepted: accepted,
		win32ExitCode:    serviceNoError,
		waitHint:         3000,
	}
	_, _, _ = procSetServiceStatus.Call(s.statusHandle, uintptr(unsafe.Pointer(&status)))
}

func (s *windowsService) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *windowsService) run() error {
	cfgData, err := os.ReadFile(DefaultConfigPath())
	if err != nil {
		return err
	}

	type runtimeConfig struct {
		Port         int    `json:"port"`
		RegistryPath string `json:"registry_path"`
		LockPath     string `json:"lock_path"`
		AdminSecret  string `json:"admin_secret"`
	}

	var cfg runtimeConfig
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.RegistryPath) == "" {
		return fmt.Errorf("server registry path is required")
	}
	if strings.TrimSpace(cfg.LockPath) == "" {
		return fmt.Errorf("server lock path is required")
	}
	if cfg.Port <= 0 {
		return fmt.Errorf("server port is invalid")
	}

	lock, err := AcquireProcessLock(cfg.LockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	srv, err := NewServer(cfg.Port, cfg.RegistryPath, cfg.AdminSecret)
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	defer srv.Stop()

	<-s.stopCh
	return nil
}

func validateInstallOptions(opts ServiceInstallOptions) error {
	if opts.Port <= 0 {
		return fmt.Errorf("server port is invalid")
	}
	if strings.TrimSpace(opts.RegistryPath) == "" {
		return fmt.Errorf("server registry path is required")
	}
	if strings.TrimSpace(opts.LockPath) == "" {
		return fmt.Errorf("server lock path is required")
	}
	if len(opts.ConfigData) == 0 {
		return fmt.Errorf("service config data is required")
	}
	return nil
}

func ensureAdmin() error {
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("administrator privileges are required")
	}
	return nil
}

func serviceExists() bool {
	return exec.Command("sc.exe", "query", ServerServiceName).Run() == nil
}

func queryServiceState() (string, error) {
	cmd := exec.Command("sc.exe", "query", ServerServiceName)
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
	return fmt.Errorf("service %s did not reach state %s (current=%s)", ServerServiceName, expected, state)
}

func stopServiceIfPresent() error {
	if !serviceExists() {
		return nil
	}

	state, err := queryServiceState()
	if err == nil && state == "STOPPED" {
		return nil
	}

	_ = runHidden("sc.exe", "stop", ServerServiceName)
	return waitForServiceState("STOPPED", 20*time.Second)
}

func startServiceAndWait() error {
	if err := runVisible("sc.exe", "start", ServerServiceName); err != nil && !strings.Contains(strings.ToLower(err.Error()), "service has already been started") {
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
