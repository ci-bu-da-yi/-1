//go:build !windows

package store

func CreateDesktopShortcut(executablePath string) error {
	return nil
	// Linux 和 macOS 不使用 Windows 桌面快捷方式。
}
