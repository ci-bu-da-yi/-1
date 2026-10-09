//go:build windows

// 只在 Windows 系统编译这个文件。
package Get

// 声明当前文件属于 Get 包。

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 通过 Windows 通知中心的 UserNotificationListener 拉取通知，
// 输出格式：通知ID<TAB>时间<TAB>标题/正文，并只保留 QQ 相关通知。
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
    Write-Error '没有通知访问权限，请开启 Windows 通知历史记录并允许 QQ 通知'
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

func queryQQNotifications() ([]string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", queryScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var s bytes.Buffer //缓冲
	cmd.Stderr = &s
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("查询通知失败: %w, s=%s", err, strings.TrimSpace(s.String()))
	}

	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(out))) //读取
	for sc.Scan() {                                        //sc.Scan的返回值是bool类型
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err() //返回是不是正常输出
}

var (
	seenMu     sync.Mutex              // seenMu：保护 seen 的并发读写
	seen       = make(map[string]bool) // seen：记录“已经处理过”的通知 ID
	maxSeenCap = 10000
)

// 分隔信息
func parseLine(line string) (QQMessage, bool) {
	parts := strings.SplitN(line, "\t", 4) //分隔4段
	if len(parts) != 4 {
		return QQMessage{}, false
	}
	return QQMessage{
		NotificationID: parts[0], //ID
		Time:           parts[1], //时间
		Title:          parts[2], //标题
		Body:           parts[3], //内容
	}, true
}

func PrimeSeen() {
	lines, err := queryQQNotifications()
	if err != nil {
		return
	}
	seenMu.Lock() //互斥锁
	defer seenMu.Unlock()
	for _, line := range lines {
		m, ok := parseLine(line)
		if ok {
			seen[m.NotificationID] = true
		} //去重
	}
}

func NextMessages(ctx context.Context, interval time.Duration) ([]QQMessage, error) { // NextMessages：持续查询，返回本轮全部新通知
	ticker := time.NewTicker(interval)
	defer ticker.Stop() //停止
	for {
		lines, err := queryQQNotifications()
		if err != nil {
			return nil, fmt.Errorf("查询通知失败: %w", err) // 返回错误，不再把错误当弹幕内容
		}
		seenMu.Lock() //互斥锁
		var newMessages []QQMessage
		for _, line := range lines {
			m, ok := parseLine(line)
			if !ok {
				continue
			}
			if !seen[m.NotificationID] {
				seen[m.NotificationID] = true
				newMessages = append(newMessages, m)
			}
		}
		if len(seen) > maxSeenCap { // 如果 seen map 超过容量上限
			clear(seen)                     // 清空整个 map，释放内存
			for _, m := range newMessages { // 重新标记本轮新消息
				seen[m.NotificationID] = true // 防止刚发现的消息被误清后重复推送
			}
		}
		seenMu.Unlock()
		if len(newMessages) > 0 {
			return newMessages, nil
		}
		select {
		case <-ctx.Done(): //上下文被取消
			return nil, ctx.Err()
		case <-ticker.C: //定时触发
		}
	}
}

func NextMessage(ctx context.Context, interval time.Duration) (QQMessage, error) {
	messages, err := NextMessages(ctx, interval)
	if err != nil {
		return QQMessage{}, err
	}
	if len(messages) == 0 {
		return QQMessage{}, fmt.Errorf("没有新的 QQ 通知")
	}
	return messages[0], nil
}
