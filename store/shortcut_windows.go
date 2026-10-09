//go:build windows

package store

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// 创建桌面快捷方式
func CreateDesktopShortcut(s string) error {
	err := ole.CoInitialize(0)
	if err != nil {
		return fmt.Errorf("COM未初始化成功；%v", err)
	}
	defer ole.CoUninitialize()
	desktopPath, err := GetDesktopPath()
	if err != nil {
		return err
	}
	lnkName := "QQ弹幕.lnk"
	lnkPath := filepath.Join(desktopPath, lnkName)
	exeDir := filepath.Dir(s)
	iconFullPath := filepath.Join(exeDir, "app.ico")
	if _, err := os.Stat(iconFullPath); os.IsNotExist(err) {
		iconFullPath = s + ",0"
	}
	WshShell, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer WshShell.Release()
	dispatch, err := WshShell.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer dispatch.Release()
	rawObj, err := oleutil.CallMethod(dispatch, "CreateShortcut", lnkPath)
	if err != nil {
		return err
	}
	link := rawObj.ToIDispatch()
	defer link.Release()
	props := []struct {
		name  string
		value interface{}
	}{
		{"TargetPath", s},
		{"WorkingDirectory", exeDir},
		{"IconLocation", iconFullPath},
		{"Description", "QQ弹幕"},
	}
	for _, prop := range props {
		if _, err := link.PutProperty(prop.name, prop.value); err != nil {
			return fmt.Errorf("设置 %s 失败: %v", prop.name, err)
		}
	}
	_, err = oleutil.CallMethod(link, "Save")
	if err != nil {
		return fmt.Errorf("save shortcut failed : %v", err)
	}
	log.Println("桌面快捷方式已创建图标：", iconFullPath)
	return nil
}
