//go:build !windows

package player

import (
	"fmt"
	"os"
	"os/exec"
)

const SupportsReady = false

func openMonitorPipe(path string) (*os.File, error) { return openPipe(path) }
func configureProcess(cmd *exec.Cmd)                {}
func openPipe(path string) (*os.File, error)        { return nil, fmt.Errorf("此平台尚未实现 mpv IPC") }
