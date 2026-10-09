package main

import (
	"context"
	"sync"
	"temp-project/Get"
	"temp-project/store"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	mu            sync.Mutex         //互斥锁
	monitorCancel context.CancelFunc //监听的取消函数，停止go_routine
}

// 用户注册接口
func (a *App) Register(username, password string) map[string]interface{} {
	user, err := store.CreateUser(username, password)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	// 注册成功后启动 QQ 通知监听，确保监听只发生在用户登录之后。
	a.startQQMonitor(user.ID)
	return map[string]interface{}{"success": true, "message": "注册成功!", "user": publicUser(user)}
}

// 用户登录接口
func (a *App) Login(username, password string) map[string]interface{} {
	user, err := store.LoginService(username, password)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	// 登录成功后启动 QQ 通知监听，未登录时不会查询系统通知。
	a.startQQMonitor(user.ID)
	return map[string]interface{}{"success": true, "message": "登录成功!", "user": publicUser(user)}
}

// 用户登出接口
func (a *App) Logout() map[string]interface{} {
	// 退出登录时停止后台监听，避免继续读取和推送 QQ 消息。
	a.stopQQMonitor()
	return map[string]interface{}{"success": true, "message": "已退出登录"}
}

func publicUser(user *store.User) map[string]interface{} {
	if user == nil {
		return nil
	}
	return map[string]interface{}{
		"id":   user.ID,
		"name": user.Name,
	}
}

/*
	func (a *App) CreateMessage(u *store.User) map[string]interface{} {
		if u == nil || u.ID <= 0 { //检验用户有效性
			return map[string]interface{}{"success": false, "message": "用户无效，请先登录"}
		}
		ctx := a.ctx //获取上下文
		if ctx == nil {
			ctx = context.Background() // 降级使用后台上下文，防止panic
		}
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second) //设置超时，防止堵塞
		defer cancel()

		msg, err := store.CreateMessages(ctx, u)
		if err != nil {
			return map[string]interface{}{"success": false, "message": err.Error()}
		}
		return map[string]interface{}{
			"success":    true,
			"message":    "保存数据成功！",
			"message_id": msg.ID,
			"group_name": msg.Title,
		}
	}
*/
func (a *App) ShowGetMessage(UserID int64, Id int) map[string]interface{} {
	message, err := store.ShowGetMessage(UserID, Id)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	return map[string]interface{}{"success": true, "message": message}
}
func (a *App) ShowGetTitle(UserID int64, Id int) map[string]interface{} {
	message, err := store.ShowGetTitle(UserID, Id)
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error()}
	}
	return map[string]interface{}{"success": true, "message": message}
}
func (a *App) startQQMonitor(userID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.monitorCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.monitorCancel = cancel

	go func() {
		Get.PrimeSeen()

		for {
			messages, err := Get.NextMessages(ctx, 3*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				runtime.EventsEmit(a.ctx, "qq:monitor-error", err.Error())
				time.Sleep(3 * time.Second)
				continue
			}
			for _, qq := range messages {
				runtime.EventsEmit(a.ctx, "qq:new-message", qq)
				//保存消息
				if _, err := AppendMessage(userID, qq); err != nil {
					runtime.EventsEmit(a.ctx, "qq:monitor-error", err.Error())
				}
			}
		}
	}()
}

func (a *App) stopQQMonitor() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.monitorCancel != nil {
		a.monitorCancel()
		a.monitorCancel = nil
	}
}
func AppendMessage(userID int64, qq Get.QQMessage) (*store.Message, error) {
	newID := int64(1)
	if len(store.Messages) > 0 {
		newID = store.Messages[len(store.Messages)-1].ID + 1
	}
	message := &store.Message{
		ID:             newID,
		UserId:         userID,
		Title:          qq.Title,
		Message:        qq.Body,
		Time:           qq.Time,
		NotificationID: qq.NotificationID,
	}
	store.Messages = append(store.Messages, *message)
	//写入文件
	if err := store.SaveMessage(); err != nil {
		return nil, err
	}

	return message, nil
}
