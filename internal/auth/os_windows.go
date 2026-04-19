//go:build windows

package auth

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	advapi32   = syscall.NewLazyDLL("advapi32.dll")
	logonUserW = advapi32.NewProc("LogonUserW")
)

const (
	logon32LogonNetwork    = 3
	logon32ProviderDefault = 0
)

// Authenticate verifies credentials against Windows local accounts via LogonUserW.
func Authenticate(username, password string) error {
	uPtr, _ := syscall.UTF16PtrFromString(username)
	// Empty domain = local machine
	dPtr, _ := syscall.UTF16PtrFromString(".")
	pPtr, _ := syscall.UTF16PtrFromString(password)

	var token syscall.Handle
	ret, _, _ := logonUserW.Call(
		uintptr(unsafe.Pointer(uPtr)),
		uintptr(unsafe.Pointer(dPtr)),
		uintptr(unsafe.Pointer(pPtr)),
		logon32LogonNetwork,
		logon32ProviderDefault,
		uintptr(unsafe.Pointer(&token)),
	)
	if ret == 0 {
		return fmt.Errorf("authentication failed")
	}
	syscall.CloseHandle(token)
	return nil
}
