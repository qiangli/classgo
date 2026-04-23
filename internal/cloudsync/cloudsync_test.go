package cloudsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"classgo/internal/models"
)

func TestGenerateRcloneConf_Drive(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := models.CloudSyncConfig{
		Enabled:            true,
		Provider:           "drive",
		ServiceAccountFile: "config/service-account.json",
		FolderID:           "abc123folderid",
	}

	confPath, err := GenerateRcloneConf(cfg, tmpDir)
	if err != nil {
		t.Fatalf("GenerateRcloneConf: %v", err)
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "[classgo]") {
		t.Error("missing [classgo] section")
	}
	if !strings.Contains(content, "type = drive") {
		t.Error("missing type = drive")
	}
	if !strings.Contains(content, "scope = drive.file") {
		t.Error("missing scope = drive.file")
	}
	if !strings.Contains(content, "root_folder_id = abc123folderid") {
		t.Error("missing root_folder_id")
	}
	// service_account_file should be resolved relative to dataDir
	expectedSA := filepath.Join(tmpDir, "config/service-account.json")
	if !strings.Contains(content, "service_account_file = "+expectedSA) {
		t.Errorf("expected service_account_file = %s, got:\n%s", expectedSA, content)
	}

	// File should be 0600
	info, _ := os.Stat(confPath)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected 0600 permissions, got %o", perm)
	}
}

func TestGenerateRcloneConf_AbsoluteServiceAccountPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := models.CloudSyncConfig{
		Enabled:            true,
		Provider:           "drive",
		ServiceAccountFile: "/etc/keys/sa.json",
		FolderID:           "xyz",
	}

	confPath, err := GenerateRcloneConf(cfg, tmpDir)
	if err != nil {
		t.Fatalf("GenerateRcloneConf: %v", err)
	}

	data, _ := os.ReadFile(confPath)
	if !strings.Contains(string(data), "service_account_file = /etc/keys/sa.json") {
		t.Error("absolute path should be preserved as-is")
	}
}

func TestGenerateRcloneConf_EmptyFields(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := models.CloudSyncConfig{
		Enabled:  true,
		Provider: "drive",
	}

	confPath, err := GenerateRcloneConf(cfg, tmpDir)
	if err != nil {
		t.Fatalf("GenerateRcloneConf: %v", err)
	}

	data, _ := os.ReadFile(confPath)
	content := string(data)
	if strings.Contains(content, "service_account_file") {
		t.Error("should not include service_account_file when empty")
	}
	if strings.Contains(content, "root_folder_id") {
		t.Error("should not include root_folder_id when empty")
	}
}

func TestGenerateRcloneConf_CreatesConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "deep", "nested")
	cfg := models.CloudSyncConfig{
		Enabled:  true,
		Provider: "drive",
	}

	confPath, err := GenerateRcloneConf(cfg, nestedDir)
	if err != nil {
		t.Fatalf("GenerateRcloneConf: %v", err)
	}

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}
}

func TestFindRclone_NotFound(t *testing.T) {
	// Set PATH to empty to ensure rclone isn't found
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir()) // empty dir, no rclone
	defer os.Setenv("PATH", origPath)

	_, err := FindRclone()
	if err == nil {
		t.Error("expected error when rclone not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestFindRclone_OnPath(t *testing.T) {
	// Create a fake rclone binary on a temp PATH
	tmpDir := t.TempDir()
	fakeRclone := filepath.Join(tmpDir, "rclone")
	os.WriteFile(fakeRclone, []byte("#!/bin/sh\n"), 0755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	path, err := FindRclone()
	if err != nil {
		t.Fatalf("expected to find rclone, got: %v", err)
	}
	if path != fakeRclone {
		t.Errorf("expected %s, got %s", fakeRclone, path)
	}
}

func TestRun_Disabled(t *testing.T) {
	cfg := models.CloudSyncConfig{Enabled: false}
	err := Run(cfg, t.TempDir())
	if err != nil {
		t.Errorf("expected nil error when disabled, got: %v", err)
	}
}

func TestRun_RcloneNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := models.CloudSyncConfig{
		Enabled:  true,
		Provider: "drive",
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	err := Run(cfg, tmpDir)
	if err == nil {
		t.Error("expected error when rclone not found")
	}
	if !strings.Contains(err.Error(), "rclone not found") {
		t.Errorf("expected 'rclone not found' error, got: %v", err)
	}
}
