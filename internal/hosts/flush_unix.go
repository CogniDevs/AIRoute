//go:build !windows

package hosts

import (
	"os/exec"
	"runtime"
)

// flushPlatformDNS issues platform-specific cache flush commands on Linux and macOS.
func flushPlatformDNS() error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("dscacheutil", "-flushcache")
		_ = cmd.Run()
		cmd2 := exec.Command("killall", "-HUP", "mDNSResponder")
		return cmd2.Run()
	case "linux":
		// Try resolvectl first
		if path, err := exec.LookPath("resolvectl"); err == nil && path != "" {
			if err := exec.Command("resolvectl", "flush-caches").Run(); err == nil {
				return nil
			}
		}
		// Fallback to systemd-resolve
		if path, err := exec.LookPath("systemd-resolve"); err == nil && path != "" {
			if err := exec.Command("systemd-resolve", "--flush-caches").Run(); err == nil {
				return nil
			}
		}
		// Fallback to nscd if present
		if path, err := exec.LookPath("nscd"); err == nil && path != "" {
			_ = exec.Command("nscd", "-i", "hosts").Run()
		}
		return nil
	default:
		return nil
	}
}
