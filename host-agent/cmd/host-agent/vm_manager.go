package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type VMManager struct {
	logger   *log.Logger
	vms      map[string]*VMInstance
	vmsMutex sync.RWMutex
	socketsDir string
}

type VMConfig struct {
	Name       string
	CPUCores   int
	RAMMB      int
	DiskGB     int
	DiskPath   string
	ISOPath    string
	GPUEnabled bool
	GPUPassthrough bool
	Display    string
	MonitorPort int
}

func NewVMManager(logger *log.Logger) *VMManager {
	socketsDir := filepath.Join(os.TempDir(), "freecompute-vms")
	os.MkdirAll(socketsDir, 0755)

	return &VMManager{
		logger:     logger,
		vms:        make(map[string]*VMInstance),
		socketsDir: socketsDir,
	}
}

func (m *VMManager) LaunchVM(config VMConfig) (*VMInstance, error) {
	vmID := fmt.Sprintf("vm_%d", time.Now().UnixNano())
	socketPath := filepath.Join(m.socketsDir, vmID+".sock")

	args := m.buildQEMUArgs(config, socketPath)
	m.logger.Printf("launching VM %s: qemu %s", vmID, args)

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to launch QEMU: %w", err)
	}

	vm := &VMInstance{
		ID:         vmID,
		Name:       config.Name,
		State:      "running",
		PID:        cmd.Process.Pid,
		SocketPath: socketPath,
		CPUCores:   config.CPUCores,
		RAMMB:      config.RAMMB,
		DiskGB:     config.DiskGB,
		GPUEnabled: config.GPUEnabled,
	}

	m.vmsMutex.Lock()
	m.vms[vmID] = vm
	m.vmsMutex.Unlock()

	go func() {
		cmd.Wait()
		m.vmsMutex.Lock()
		if v, ok := m.vms[vmID]; ok {
			v.State = "stopped"
		}
		m.vmsMutex.Unlock()
	}()

	m.logger.Printf("VM %s launched (PID=%d)", vmID, cmd.Process.Pid)
	return vm, nil
}

func (m *VMManager) StopVM(vmID string) error {
	m.vmsMutex.Lock()
	vm, ok := m.vms[vmID]
	m.vmsMutex.Unlock()

	if !ok {
		return fmt.Errorf("VM %s not found", vmID)
	}

	cmd := exec.Command("qemu-system-x86_64", "-monitor", "unix:"+vm.SocketPath+",server,nowait", "-system-powerdown")
	if err := cmd.Run(); err != nil {
		proc, _ := os.FindProcess(vm.PID)
		if proc != nil {
			proc.Signal(os.Interrupt)
		}
	}

	m.logger.Printf("VM %s stopped", vmID)
	return nil
}

func (m *VMManager) DestroyVM(vmID string) error {
	m.StopVM(vmID)

	m.vmsMutex.Lock()
	delete(m.vms, vmID)
	m.vmsMutex.Unlock()

	socketPath := filepath.Join(m.socketsDir, vmID+".sock")
	os.Remove(socketPath)

	m.logger.Printf("VM %s destroyed", vmID)
	return nil
}

func (m *VMManager) ListVMs() []VMInstance {
	m.vmsMutex.RLock()
	defer m.vmsMutex.RUnlock()

	vms := make([]VMInstance, 0, len(m.vms))
	for _, vm := range m.vms {
		vms = append(vms, *vm)
	}
	return vms
}

func (m *VMManager) GetVM(vmID string) (*VMInstance, error) {
	m.vmsMutex.RLock()
	defer m.vmsMutex.RUnlock()

	vm, ok := m.vms[vmID]
	if !ok {
		return nil, fmt.Errorf("VM %s not found", vmID)
	}
	return vm, nil
}

func (m *VMManager) buildQEMUArgs(config VMConfig, socketPath string) []string {
	args := []string{
		"-name", config.Name,
		"-m", fmt.Sprintf("%d", config.RAMMB),
		"-smp", fmt.Sprintf("%d", config.CPUCores),
		"-enable-kvm",
		"-cpu", "host",
		"-machine", "q35,accel=kvm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", config.DiskPath),
		"-monitor", fmt.Sprintf("unix:%s,server,nowait", socketPath),
		"-display", "none",
		"-vga", "virtio",
		"-device", "virtio-net-pci,netdev=net0",
		"-netdev", "user,id=net0,hostfwd=tcp::2222-:22",
		"-device", "virtio-serial-pci",
		"-chardev", "socket,path=/tmp/freecompute-agent.sock,server=on,wait=off,id=agent0",
		"-device", "virtserialport,chardev=agent0,name=com.freecompute.agent.0",
		"-device", "virtio-rng-pci",
		"-snapshot",
	}

	if config.ISOPath != "" {
		args = append(args, "-cdrom", config.ISOPath)
	}

	if config.GPUPassthrough {
		args = append(args,
			"-device", "vfio-pci,host=01:00.0,multifunction=on",
			"-device", "vfio-pci,host=01:00.1",
		)
	}

	if config.MonitorPort > 0 {
		args = append(args, "-monitor", fmt.Sprintf("tcp:127.0.0.1:%d,server,nowait", config.MonitorPort))
	}

	return args
}
