//go:build windows

package hosts

import (
	"os/exec"
	"syscall"
)

// flushPlatformDNS issues ipconfig /flushdns with a hidden console window.
func flushPlatformDNS() error {
	cmd := exec.Command("ipconfig", "/flushdns")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
