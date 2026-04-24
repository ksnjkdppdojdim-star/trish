//go:build windows

package agent

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	serviceWin32OwnProcess       = 0x00000010
	serviceStopPending           = 0x00000003
	serviceRunning               = 0x00000004
	serviceStartPending          = 0x00000002
	serviceStopped               = 0x00000001
	serviceAcceptStop            = 0x00000001
	serviceAcceptShutdown        = 0x00000004
	serviceControlStop           = 0x00000001
	serviceControlShutdown       = 0x00000005
	serviceNoError               = 0x00000000
)

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

var (
	modAdvapi32                   = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcher = modAdvapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandler = modAdvapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus          = modAdvapi32.NewProc("SetServiceStatus")

	currentService     *windowsService
	currentServiceOnce sync.Once
)

type windowsService struct {
	statusHandle uintptr
	stopOnce     sync.Once
	stopCh       chan struct{}
	doneCh       chan struct{}
	runErr       error
}

// RunAsWindowsService lance l'agent en tant que vrai service Windows.
func RunAsWindowsService() error {
	currentServiceOnce = sync.Once{}
	currentService = &windowsService{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	namePtr, err := syscall.UTF16PtrFromString(ServiceName)
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
		namePtr, _ := syscall.UTF16PtrFromString(ServiceName)
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
		return 0
	default:
		return 0
	}
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
	cfg, err := LoadConfig(DefaultConfigPath())
	if err != nil {
		return err
	}

	logger, closer, err := OpenLogger(cfg)
	if err != nil {
		return err
	}
	defer closer.Close()

	logger.Printf("service starting")
	agentSvc, err := StartWithConfig(cfg, logger)
	if err != nil {
		logger.Printf("service startup failed: %v", err)
		return err
	}

	<-s.stopCh
	logger.Printf("service stopping")
	agentSvc.Stop()
	time.Sleep(1 * time.Second)
	return nil
}

// RunForeground execute l'agent au premier plan pour le debug et les tests.
func RunForeground(cfg *Config) error {
	logger, closer, err := OpenLogger(cfg)
	if err != nil {
		return err
	}
	defer closer.Close()

	agentSvc, err := StartWithConfig(cfg, logger)
	if err != nil {
		return err
	}

	fmt.Println("=== Trish Agent ===")
	fmt.Printf("Hostname: %s\n", agentSvc.Agent().Hostname)
	fmt.Printf("Server: %s:%d\n", cfg.ServerAddr, cfg.ServerPort)
	fmt.Println("Mode: foreground runtime")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	agentSvc.Stop()
	return nil
}

// RunServiceMode route l'execution vers le mode service Windows.
func RunServiceMode() error {
	return RunAsWindowsService()
}

func init() {
	log.SetFlags(log.LstdFlags | log.LUTC)
}
