//go:build darwin

package auth

import (
	"fmt"
	"os/exec"
)

// Authenticate verifies credentials against macOS Directory Services.
func Authenticate(username, password string) error {
	cmd := exec.Command("/usr/bin/dscl", ".", "-authonly", username, password)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("authentication failed")
	}
	return nil
}
