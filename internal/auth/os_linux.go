//go:build linux

package auth

import (
	"fmt"
	"os/exec"
	"strings"
)

// Authenticate verifies credentials against Linux PAM via unix_chkpwd.
// unix_chkpwd is a setuid helper used by PAM, available on virtually all Linux systems.
func Authenticate(username, password string) error {
	// Try common paths for unix_chkpwd
	paths := []string{"/sbin/unix_chkpwd", "/usr/sbin/unix_chkpwd"}
	var chkpwd string
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			chkpwd = p
			break
		}
	}
	if chkpwd == "" {
		// Fallback: look in PATH
		var err error
		chkpwd, err = exec.LookPath("unix_chkpwd")
		if err != nil {
			return fmt.Errorf("unix_chkpwd not found; install the pam package")
		}
	}

	cmd := exec.Command(chkpwd, username, "nullok")
	cmd.Stdin = strings.NewReader(password)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("authentication failed")
	}
	return nil
}
