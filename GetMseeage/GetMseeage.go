package GetMessage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"my-project/user"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Powershell脚本
const queryScript = `
$utf8 = New-Object System.Text.UTF8Encoding($false)
[Console]::OutputEncoding = $utf8
$OutputEncoding = $utf8

$listenerType = [Windows.UI.Notifications.Management.UserNotificationListener, Windows.UI.Notifications.Management, ContentType = WindowsRuntime]
Add-Type -AssemblyName System.Runtime.WindowsRuntime

$asyncOperationTypeName = 'IAsyncOperation' + [char]96 + '1'
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
$_.Name -eq 'AsTask' -and
$_.GetParameters().Count -eq 1 -and
$_.GetParameters()[0].ParameterType.Name -eq $asyncOperationTypeName
})[0]

function Await($WinRtTask, $ResultType) {
$asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
$netTask = $asTask.Invoke($null, @($WinRtTask))
$netTask.Wait(-1) | Out-Null
$netTask.Result
}

$listener = [Windows.UI.Notifications.Management.UserNotificationListener]::Current
$accessOperation = $listener.RequestAccessAsync()
$status = Await $accessOperation ([Windows.UI.Notifications.Management.UserNotificationListenerAccessStatus])
if ($status -ne 'Allowed') {
    Write-Error "没有通知访问权限，当前状态：$status。请开启 Windows 通知历史和通知访问权限"
    exit 1
}

$notificationsOperation = $listener.GetNotificationsAsync([Windows.UI.Notifications.NotificationKinds]::Toast)
$all = Await $notificationsOperation ([System.Collections.Generic.IReadOnlyList[Windows.UI.Notifications.UserNotification]])

foreach ($n in $all) {
$app = ''
if ($null -ne $n.AppInfo -and $null -ne $n.AppInfo.DisplayInfo) {
$app = [string]$n.AppInfo.DisplayInfo.DisplayName
}
if ($app -notmatch '^(QQ|QQNT|腾讯QQ)$') { continue }

$texts = @()
if ($null -ne $n.Notification -and $null -ne $n.Notification.Visual) {
foreach ($binding in $n.Notification.Visual.Bindings) {
foreach ($t in $binding.GetTextElements()) {
$texts += [string]$t.Text
}
}
}

$title = ''
$body = ''
if ($texts.Count -ge 1) {
$title = $texts[0]
}
if ($texts.Count -ge 2) {
$body = ($texts[1..($texts.Count-1)] -join ' ')
} elseif ($texts.Count -eq 1) {
$body = $texts[0]
}

"{0}{1}{2:yyyy-MM-dd HH:mm:ss}{1}{3}{1}{4}" -f $n.Id, [char]9, $n.CreationTime.LocalDateTime, $title, $body
}`

var (
	Messages []Message
)

type Message struct {
	ID             int64  `json:"ID"`
	UserID         int64  `json:"UserID"`
	NotificationID string `json:"notification_id"` //通知ID
	Time           string `json:"time"`            //通知时间
	Title          string `json:"title"`           //群名
	Body           string `json:"body"`            //通知内容
}

func init() {
	if err := LoadMessage(); err != nil {
		fmt.Println("加载数据失败！", err)
	}
}
func LoadMessage() error {
	data, err := os.ReadFile(user.MessageFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			Messages = []Message{}
			return nil
		}
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		Messages = []Message{}
		return nil
	}
	if err := json.Unmarshal(data, &Messages); err != nil {
		if Messages == nil {
			Messages = []Message{}
		}
		return nil
	}
	var legacy Message
	if err := json.Unmarshal(data, &legacy); err != nil {
		if legacy.ID == 0 && legacy.Title == "" && legacy.Body == "" {
			Messages = []Message{}
		} else {
			Messages = []Message{legacy}
		}
		return nil
	}
	return fmt.Errorf("消息格式错误！")
}
func SaveMessage() error {
	//反序列化
	data, err := json.MarshalIndent(Messages, "", " ")
	if err != nil {
		return err
	}
	//序列化后写入文件
	return os.WriteFile(user.MessageFilePath, data, 0644)
}

// 数据源头
func queryQQNotifications() ([]string, error) {
	//隐藏Powershell
	cmd := exec.Command("powershell", "-Noprofile", "-ExecutionPolicy", "Bypass", "Command", queryScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr //命令加入到缓冲
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf(
			"查询通知失败: %w, 详细信息: %s",
			err,
			detail,
		)
	}

	var lines []string
	sc := bufio.NewScanner(strings.NewReader(stdout.String()))
	for sc.Scan() { //都是否有下一行
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err() //可能的错误
}

var (
	seenMu     sync.Mutex              //互斥锁
	seen       = make(map[string]bool) //哈希表去重
	maxSeenCap = 10000                 //最大容量
)

func parseLine(line string) (Message, bool) {
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) != 4 {
		return Message{}, false
	}
	return Message{
		NotificationID: parts[0],
		Time:           parts[1],
		Title:          parts[2],
		Body:           parts[3],
	}, true
}

// 区分已读和未读的消息
func PrimeSeen() error {
	lines, err := queryQQNotifications()
	if err != nil {
		return err
	}
	seenMu.Lock()
	defer seenMu.Unlock()
	//处理每一条消息
	for _, line := range lines {
		msg, ok := parseLine(line)
		if ok {
			seen[msg.NotificationID] = true
		}
	}
	return nil
}
func NextMessages(ctx context.Context, interval time.Duration) ([]Message, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop() //函数退出时停止定时器
	for {
		lines, err := queryQQNotifications()
		if err != nil {
			return nil, fmt.Errorf("查询失败！: %w", err)
		}
		seenMu.Lock()
		var newMessages []Message
		for _, line := range lines {
			msg, ok := parseLine(line)
			if !ok {
				continue
			}
			if !seen[msg.NotificationID] {
				seen[msg.NotificationID] = true
				newMessages = append(newMessages, msg)
			}
		}
		if len(newMessages) > maxSeenCap {
			clear(seen)
			for _, line := range lines {
				if m, ok := parseLine(line); ok {
					seen[m.NotificationID] = true
				}
			}
		}
		seenMu.Unlock()
		if len(newMessages) > 0 {
			return newMessages, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
