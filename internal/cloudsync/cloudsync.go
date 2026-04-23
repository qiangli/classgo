package cloudsync

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"classgo/internal/models"
)

// dirs lists the subdirectories under DataDir to sync to the remote.
var dirs = []string{"backups", "attendances"}

// Run is the top-level entry point called by the cron job.
// It generates rclone config, locates the binary, and syncs each directory.
func Run(cfg models.CloudSyncConfig, dataDir string) error {
	if !cfg.Enabled {
		return nil
	}

	confPath, err := GenerateRcloneConf(cfg, dataDir)
	if err != nil {
		return fmt.Errorf("generate rclone config: %w", err)
	}

	rclonePath, err := FindRclone()
	if err != nil {
		return fmt.Errorf("rclone not found: %w", err)
	}

	var errs []string
	for _, dir := range dirs {
		localDir := filepath.Join(dataDir, dir)
		if _, err := os.Stat(localDir); os.IsNotExist(err) {
			continue // skip directories that don't exist yet
		}
		remoteDest := fmt.Sprintf("classgo:%s", dir)
		if err := sync(rclonePath, confPath, localDir, remoteDest); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", dir, err))
			log.Printf("Cloud sync failed for %s: %v", dir, err)
		} else {
			log.Printf("Cloud sync completed for %s", dir)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// sync runs rclone copy for a single local directory to a remote destination.
func sync(rclonePath, confPath, localDir, remoteDest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	args := []string{
		"copy",
		localDir,
		remoteDest,
		"--config", confPath,
		"--retries", "3",
		"--low-level-retries", "10",
		"--stats-log-level", "NOTICE",
		"--log-level", "INFO",
	}

	cmd := exec.CommandContext(ctx, rclonePath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if out := strings.TrimSpace(stderr.String()); out != "" {
		log.Printf("Cloud sync [%s]: %s", filepath.Base(localDir), out)
	}
	if err != nil {
		return fmt.Errorf("rclone copy: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// GenerateRcloneConf writes an rclone.conf file under dataDir/config/ based on
// the cloud sync configuration. Returns the path to the generated config file.
func GenerateRcloneConf(cfg models.CloudSyncConfig, dataDir string) (string, error) {
	confDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	// Resolve service account file path relative to dataDir
	saFile := cfg.ServiceAccountFile
	if saFile != "" && !filepath.IsAbs(saFile) {
		saFile = filepath.Join(dataDir, saFile)
	}

	var buf strings.Builder
	buf.WriteString("[classgo]\n")
	fmt.Fprintf(&buf, "type = %s\n", cfg.Provider)

	switch cfg.Provider {
	case "drive":
		buf.WriteString("scope = drive.file\n")
		if saFile != "" {
			fmt.Fprintf(&buf, "service_account_file = %s\n", saFile)
		}
		if cfg.FolderID != "" {
			fmt.Fprintf(&buf, "root_folder_id = %s\n", cfg.FolderID)
		}
	default:
		if saFile != "" {
			fmt.Fprintf(&buf, "service_account_file = %s\n", saFile)
		}
	}

	confPath := filepath.Join(confDir, "rclone.conf")
	if err := os.WriteFile(confPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("write rclone.conf: %w", err)
	}
	return confPath, nil
}

// FindRclone locates the rclone binary. It checks:
// 1. bin/rclone next to the running executable
// 2. rclone on PATH
func FindRclone() (string, error) {
	// Check next to our own executable
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "rclone")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Check PATH
	if p, err := exec.LookPath("rclone"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("rclone binary not found (checked bin/ and PATH)")
}
