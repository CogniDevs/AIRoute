package hosts

import (
	"os/exec"
	"runtime"
)

// FlushDNS issues the platform-specific cache flush command.
func FlushDNS() error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("ipconfig", "/flushdns")
		return cmd.Run()
	case "darwin":
		cmd := exec.Command("dscacheutil", "-flushcache")
		_ = cmd.Run()
		cmd2 := exec.Command("killall", "-HUP", "mDNSResponder")
		return cmd2.Run()
	case "linux":
		cmd := exec.Command("resolvectl", "flush-caches")
		if err := cmd.Run(); err != nil {
			cmd2 := exec.Command("systemd-resolve", "--flush-caches")
			return cmd2.Run()
		}
		return nil
	default:
		return nil
	}
}
