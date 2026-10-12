//go:build windows

package downloads

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		tool := filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe")
		return exec.Command(tool, "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
