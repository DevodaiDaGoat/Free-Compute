package config

import "os"

type Config struct {
	HostID        string
	HostName      string
	Region        string
	GatewayURL    string
	CertPath      string // Path to TLS client certificate
	KeyPath       string // Path to TLS private key
	CAPath        string // Path to CA certificate for gateway verification
	QEMUBinary    string
	StoragePath   string
	MaxVMs        int
}

func Load() *Config {
	return &Config{
		HostID:      getEnv("HOST_ID", ""),
		HostName:    getEnv("HOST_NAME", ""),
		Region:      getEnv("HOST_REGION", "unknown"),
		GatewayURL:  getEnv("GATEWAY_URL", "https://api.freecompute.io"),
		CertPath:    getEnv("TLS_CERT_PATH", "/etc/freecompute/host.crt"),
		KeyPath:     getEnv("TLS_KEY_PATH", "/etc/freecompute/host.key"),
		CAPath:      getEnv("TLS_CA_PATH", "/etc/freecompute/ca.crt"),
		QEMUBinary:  getEnv("QEMU_BINARY", "/usr/bin/qemu-system-x86_64"),
		StoragePath: getEnv("STORAGE_PATH", "/var/lib/freecompute/vms"),
		MaxVMs:      10,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
