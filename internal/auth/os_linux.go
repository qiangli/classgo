//go:build linux

package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/md5_crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
)

var errAuthFailed = errors.New("authentication failed")

// Authenticate verifies credentials against Linux PAM via unix_chkpwd, with
// a direct /etc/shadow fallback. The fallback exists because Ubuntu 26.04
// ships PAM 1.7.0 with a unix_chkpwd helper that rejects valid passwords;
// reading /etc/shadow ourselves requires the process to be in the shadow
// group (or run as root).
func Authenticate(username, password string) error {
	if err := authChkpwd(username, password); err == nil {
		return nil
	}
	return authShadow(username, password)
}

// authChkpwd shells out to unix_chkpwd, the standard PAM helper.
func authChkpwd(username, password string) error {
	var chkpwd string
	for _, p := range []string{"/sbin/unix_chkpwd", "/usr/sbin/unix_chkpwd"} {
		if _, err := exec.LookPath(p); err == nil {
			chkpwd = p
			break
		}
	}
	if chkpwd == "" {
		var err error
		if chkpwd, err = exec.LookPath("unix_chkpwd"); err != nil {
			return fmt.Errorf("unix_chkpwd not found; install the pam package")
		}
	}

	cmd := exec.Command(chkpwd, username, "nullok")
	cmd.Stdin = strings.NewReader(password)
	if err := cmd.Run(); err != nil {
		return errAuthFailed
	}
	return nil
}

// authShadow reads /etc/shadow directly and verifies the password against
// the stored crypt(3) hash. Requires shadow-group membership to read the file.
func authShadow(username, password string) error {
	f, err := os.Open("/etc/shadow")
	if err != nil {
		return errAuthFailed
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if len(parts) < 2 || parts[0] != username {
			continue
		}
		hash := parts[1]
		if hash == "" || hash[0] == '!' || hash[0] == '*' {
			return errAuthFailed
		}
		c := crypt.NewFromHash(hash)
		if c == nil {
			return errAuthFailed
		}
		if err := c.Verify(hash, []byte(password)); err != nil {
			return errAuthFailed
		}
		return nil
	}
	return errAuthFailed
}
