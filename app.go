package main

import (
	"context"
	"fmt"
	"log"
	GetMessage "my-project/GetMseeage"
	"my-project/user"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	mu            sync.Mutex
	monitorCancel context.CancelFunc //监听
}

// 返回用户和密码
func publicUser(user *user.User) map[string]interface{} {
	if user == nil {
		return nil
	}
	return map[string]interface{}{
		"id":   user.ID,
		"user": user.Name,
	}
}

// 开始监听
func (a *App) startQQMonitor(userID int64) {
	a.mu.Lock() //互斥锁
	defer a.mu.Unlock()

	if a.monitorCancel != nil {
		return
	}
	//context.WithCancel取消上下文
	//退出监听
	ctx, cancel := context.WithCancel(a.ctx)
	a.monitorCancel = cancel

	go func() {
		if err := GetMessage.PrimeSeen(); err != nil {
			runtime.EventsEmit(
				a.ctx,
				"qq:monitor-error",
				fmt.Errorf("初始化消息监听失败: %w", err).Error(),
			)
		}

		for {
			messages, err := GetMessage.NextMessages(ctx, 3*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				runtime.EventsEmit(a.ctx, "qq:monitor-error", err.Error())
				select {
				case <-a.ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
			for _, qq := range messages {
				//发送给前端
				runtime.EventsEmit(a.ctx, "qq:new-message", qq)
				//保存消息
				if _, err := AppendMessage(userID, qq); err != nil {
					runtime.EventsEmit(a.ctx, "qq:monitor-error", err.Error())
				}
			}
		}
	}()
}

// 停止监听
func (a *App) stopQQMonitor() {
	a.mu.Lock()
	defer a.mu.Unlock()
	//monitorCancel是取消函数！
	if a.monitorCancel != nil {
		a.monitorCancel()
		a.monitorCancel = nil
	}
}

// 用户注册接口
func (a *App) Register(username, password string) map[string]interface{} {
	user1, err := user.CreateUser(username, password)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	if user1 == nil {
		return map[string]interface{}{"success": false, "message": "用户错误！"}
	}
	// 注册成功后启动 QQ 通知监听，确保监听只发生在用户登录之后。
	a.startQQMonitor(user1.ID)
	return map[string]interface{}{"success": true, "message": "注册成功!", "user": publicUser(user1)}
}

// 用户登录接口
func (a *App) Login(username, password string) map[string]interface{} {
	user2, err := user.LoginService(username, password)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	// 登录成功后启动 QQ 通知监听，未登录时不会查询系统通知。
	a.startQQMonitor(user2.ID)
	return map[string]interface{}{"success": true, "message": "登录成功!", "user": publicUser(user2)}
}

// 用户登出接口
func (a *App) Logout() map[string]interface{} {
	// 退出登录时停止后台监听，避免继续读取和推送 QQ 消息。
	a.stopQQMonitor()
	return map[string]interface{}{"success": true, "message": "已退出登录"}
}

// 添加新的消息
func AppendMessage(userID int64, qq GetMessage.Message) (*GetMessage.Message, error) {
	newID := int64(1)
	if len(GetMessage.Messages) > 0 {
		newID = GetMessage.Messages[len(GetMessage.Messages)-1].ID + 1
	}
	message := &GetMessage.Message{
		ID:             newID,
		UserID:         userID,
		NotificationID: qq.NotificationID,
		Time:           qq.Time,
		Title:          qq.Title,
		Body:           qq.Body,
	}
	GetMessage.Messages = append(GetMessage.Messages, *message)
	//写入文件
	if err := GetMessage.SaveMessage(); err != nil {
		return nil, err
	}

	return message, nil
}

// 创建桌面快捷方式
func CreateDesktopShortcut(s string) error {
	err := ole.CoInitialize(0)
	if err != nil {
		return fmt.Errorf("COM未初始化成功；%v", err)
	}
	defer ole.CoUninitialize()
	desktopPath, err := user.GetDeskPath()
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
