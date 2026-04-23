package tunnel

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"classgo/internal/models"
)

// Start locates frpc, generates a config file, and launches frpc as a subprocess.
// The returned *exec.Cmd can be used to stop the process later.
func Start(cfg models.TunnelConfig, dataDir string) (*exec.Cmd, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	frpcPath, err := FindFrpc()
	if err != nil {
		return nil, err
	}

	confPath, err := GenerateConfig(cfg, dataDir)
	if err != nil {
		return nil, fmt.Errorf("generate frpc config: %w", err)
	}

	cmd := exec.Command(frpcPath, "-c", confPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start frpc: %w", err)
	}

	log.Printf("Tunnel started (PID %d) → %s", cmd.Process.Pid, PublicURL(cfg))
	return cmd, nil
}

// Stop gracefully terminates the frpc subprocess.
func Stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	log.Printf("Stopping tunnel (PID %d)...", cmd.Process.Pid)
	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
}

// PublicURL returns the expected public URL based on the tunnel config.
func PublicURL(cfg models.TunnelConfig) string {
	if cfg.Domain != "" {
		return fmt.Sprintf("http://%s", cfg.Domain)
	}
	host, _, _ := net.SplitHostPort(cfg.ServerAddr)
	return fmt.Sprintf("http://%s", host)
}

// GenerateConfig writes a frpc.toml configuration file to {dataDir}/config/frpc.toml.
func GenerateConfig(cfg models.TunnelConfig, dataDir string) (string, error) {
	confDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	host, portStr, err := net.SplitHostPort(cfg.ServerAddr)
	if err != nil {
		return "", fmt.Errorf("invalid server_addr %q (expected host:port): %w", cfg.ServerAddr, err)
	}
	serverPort, err := strconv.Atoi(portStr)
	if err != nil {
		return "", fmt.Errorf("invalid port in server_addr: %w", err)
	}

	localPort := cfg.LocalPort
	if localPort == 0 {
		localPort = 8080
	}

	var b strings.Builder
	fmt.Fprintf(&b, "serverAddr = %q\n", host)
	fmt.Fprintf(&b, "serverPort = %d\n", serverPort)
	if cfg.Token != "" {
		fmt.Fprintf(&b, "\n[auth]\ntoken = %q\n", cfg.Token)
	}
	fmt.Fprintf(&b, "\n[[proxies]]\n")
	fmt.Fprintf(&b, "name = \"classgo-web\"\n")
	fmt.Fprintf(&b, "type = \"http\"\n")
	fmt.Fprintf(&b, "localPort = %d\n", localPort)
	if cfg.Domain != "" {
		fmt.Fprintf(&b, "customDomains = [%q]\n", cfg.Domain)
	}

	confPath := filepath.Join(confDir, "frpc.toml")
	if err := os.WriteFile(confPath, []byte(b.String()), 0600); err != nil {
		return "", fmt.Errorf("write frpc.toml: %w", err)
	}
	return confPath, nil
}

// FindFrpc locates the frpc binary. It checks:
// 1. bin/frpc next to the running executable
// 2. frpc on PATH
func FindFrpc() (string, error) {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "frpc")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if p, err := exec.LookPath("frpc"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("frpc binary not found (checked bin/ and PATH)")
}
