//go:build windows

package hosts

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = shell32.NewProc("ShellExecuteExW")
)

const (
	seeMaskNoCloseProcess = 0x00000040
	swHide                = 0
)

type shellExecuteInfoW struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       windows.Handle
}

// writeElevated relaunches the current binary with elevated privileges (UAC runas)
// passing the --apply-hosts flag pointing to tmpPath.
func writeElevated(tmpPath string, hostsPath string) error {
	selfExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к исполняемому файлу: %w", err)
	}

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exePtr, err := syscall.UTF16PtrFromString(selfExe)
	if err != nil {
		return err
	}
	argsPtr, err := syscall.UTF16PtrFromString(fmt.Sprintf("--apply-hosts %q", tmpPath))
	if err != nil {
		return err
	}

	var info shellExecuteInfoW
	info.cbSize = uint32(unsafe.Sizeof(info))
	info.fMask = seeMaskNoCloseProcess
	info.lpVerb = verbPtr
	info.lpFile = exePtr
	info.lpParameters = argsPtr
	info.nShow = swHide

	ret, _, callErr := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return fmt.Errorf("запрос прав Администратора (UAC) отклонён или не удался: %v", callErr)
	}

	if info.hProcess != 0 {
		defer windows.CloseHandle(info.hProcess)
		event, err := windows.WaitForSingleObject(info.hProcess, windows.INFINITE)
		if err != nil {
			return fmt.Errorf("ошибка ожидания повышенного процесса: %w", err)
		}
		if event == windows.WAIT_OBJECT_0 {
			var exitCode uint32
			if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err == nil && exitCode != 0 {
				return fmt.Errorf("применение hosts завершилось с кодом ошибки: %d", exitCode)
			}
		}
	}

	return nil
}
