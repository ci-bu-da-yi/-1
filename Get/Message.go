package Get

type QQMessage struct {
	NotificationID string `json:"notification_id"` //通知ID
	Time           string `json:"time"`            //通知时间
	Title          string `json:"title"`           //群名
	Body           string `json:"body"`            //通知内容
}
