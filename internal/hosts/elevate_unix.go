//go:build !windows

package hosts

import (
	"fmt"
	"os/exec"
	"runtime"
)

// writeElevated writes the temp file content into target hosts file using OS privilege escalation.
func writeElevated(tmpPath string, hostsPath string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`do shell script "cat %q > %q" with administrator privileges`, tmpPath, hostsPath)
		cmd := exec.Command("osascript", "-e", script)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ошибка повышения прав macOS (osascript): %v (вывод: %s)", err, string(out))
		}
		return nil

	case "linux":
		pkexecPath, err := exec.LookPath("pkexec")
		if err != nil || pkexecPath == "" {
			return fmt.Errorf("для записи в %s требуются права root. Утилита pkexec (Polkit) не найдена в системе. Пожалуйста, установите polkit или настройте права доступа к файлу", hostsPath)
		}

		cmd := exec.Command("pkexec", "sh", "-c", `cat "$1" > "$2"`, "_", tmpPath, hostsPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("запрос прав root через pkexec отклонён или завершился с ошибкой: %v (%s)", err, string(out))
		}
		return nil

	default:
		return fmt.Errorf("автоматическое повышение прав не поддерживается для платформы %s", runtime.GOOS)
	}
}
