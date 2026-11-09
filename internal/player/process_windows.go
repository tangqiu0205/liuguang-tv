//go:build windows

package player

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

const SupportsReady = true

func openPipe(path string) (*os.File, error) { return openPipeAccess(path, syscall.GENERIC_WRITE) }
func openMonitorPipe(path string) (*os.File, error) {
	return openPipeAccess(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE)
}
func openPipeAccess(path string, access uint32) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// Identification-only SQOS prevents a pipe server from impersonating this client.
	h, err := syscall.CreateFile(name, access, 0, nil, syscall.OPEN_EXISTING, 0x00100000|0x00010000, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
