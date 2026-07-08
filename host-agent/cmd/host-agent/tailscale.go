package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type TailscaleManager struct {
	logger      *log.Logger
	mu          sync.Mutex
	hostTailIP  string
	hostName    string
	vms         map[string]string // vmID -> tailscale IP
	gatewayURL  string
	token       string
}

type TailscaleHostInfo struct {
	TailscaleIP string `json:"tailscaleIp"`
	HostName    string `json:"hostName"`
	VMs         []VMInfo `json:"vms"`
}

type VMInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	TailIP  string `json:"tailIp,omitempty"`
	PortMap map[int]int `json:"portMap,omitempty"`
}

func NewTailscaleManager(logger *log.Logger, gatewayURL, token string) *TailscaleManager {
	return &TailscaleManager{
		logger:     logger,
		vms:        make(map[string]string),
		gatewayURL: gatewayURL,
		token:      token,
	}
}

func (m *TailscaleManager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ip, err := getTailscaleIP()
	if err != nil {
		m.logger.Printf("tailscale not found or not running: %v", err)
		m.logger.Printf("tailscale is optional. TCP/UDP will use WebSocket tunnel instead")
		return nil
	}

	name, _ := getTailscaleHostname()
	m.hostTailIP = ip
	m.hostName = name
	m.logger.Printf("tailscale detected: IP=%s hostname=%s", ip, name)
	return nil
}

func (m *TailscaleManager) HostIP() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hostTailIP
}

func (m *TailscaleManager) RegisterVMIP(vmID, vmTailIP string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vms[vmID] = vmTailIP
	m.logger.Printf("vm %s registered with tailscale IP %s", vmID, vmTailIP)
}

func (m *TailscaleManager) GetVMIP(vmID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vms[vmID]
}

func (m *TailscaleManager) RegisterWithGateway() error {
	m.mu.Lock()
	hostIP := m.hostTailIP
	hostName := m.hostName
	vms := make([]VMInfo, 0, len(m.vms))
	for vmID, tailIP := range m.vms {
		vms = append(vms, VMInfo{ID: vmID, TailIP: tailIP})
	}
	m.mu.Unlock()

	if hostIP == "" {
		return nil
	}

	info := TailscaleHostInfo{
		TailscaleIP: hostIP,
		HostName:    hostName,
		VMs:         vms,
	}

	body, err := json.Marshal(info)
	if err != nil {
		return err
	}

	registerURL := strings.TrimRight(m.gatewayURL, "/") + "/tailscale/register"
	req, err := http.NewRequest(http.MethodPost, registerURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		m.logger.Printf("register with gateway via tailscale: %v", err)
		return err
	}
	defer resp.Body.Close()

	m.logger.Printf("registered tailscale IP %s with gateway (status=%s)", hostIP, resp.Status)
	return nil
}

func getTailscaleIP() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "ip", "-4")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tailscale not available: %w", err)
	}
	ip := strings.TrimSpace(string(out))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid tailscale IP: %s", ip)
	}
	return ip, nil
}

func getTailscaleHostname() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var status struct {
		Self struct {
			HostName string `json:"HostName"`
			DNSName  string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", err
	}
	if status.Self.HostName != "" {
		return status.Self.HostName, nil
	}
	return strings.TrimSuffix(status.Self.DNSName, "."), nil
}

func EnsureTailscale() bool {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "status")
	return cmd.Run() == nil
}
