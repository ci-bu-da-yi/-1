package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var usersFilePath string
var messageFilePath string
var Users []User
var Messages []Message

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
}
type Message struct {
	ID             int64  `json:"id"`
	UserId         int64  `json:"userid"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	Time           string `json:"time,omitempty"`
	NotificationID string `json:"notification_id,omitempty"`
}

// 启动时自己启动
func init() {
	initStoragePaths()
	LoadUser()
	if err := LoadMessage(); err != nil {
		fmt.Println("加载数据失败！", err)
	}
}

func initStoragePaths() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		usersFilePath = filepath.Join("store", "users.json")
		messageFilePath = filepath.Join("store", "message.json")
		return
	}

	dataDir := filepath.Join(configDir, "QQDanmaku")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		usersFilePath = filepath.Join("store", "users.json")
		messageFilePath = filepath.Join("store", "message.json")
		return
	}

	usersFilePath = filepath.Join(dataDir, "users.json")
	messageFilePath = filepath.Join(dataDir, "message.json")
}
func LoadUser() {
	data, err := os.ReadFile(usersFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("用户数据解析失败！")
		}
		Users = []User{}
		return
	}
	if len(bytes.TrimSpace(data)) == 0 {
		Users = []User{}
		return
	}
	if err := json.Unmarshal(data, &Users); err == nil {
		if Users == nil {
			Users = []User{}
		}
		return
	}

	// 兼容旧版本单用户对象格式。
	var legacyUser User
	if err := json.Unmarshal(data, &legacyUser); err != nil {
		fmt.Println("解析用户数据失败！")
		Users = []User{}
		return
	}
	if legacyUser.ID == 0 && legacyUser.Name == "" && legacyUser.Password == "" {
		Users = []User{}
		return
	}
	Users = []User{legacyUser}
}
func SaveUser() error {
	data, err := json.MarshalIndent(Users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(usersFilePath, data, 0644)
}
func LoadMessage() error {
	data, err := os.ReadFile(messageFilePath)
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
	if err := json.Unmarshal(data, &Messages); err == nil {
		if Messages == nil {
			Messages = []Message{}
		}
		return nil
	}
	var legacy Message
	if err := json.Unmarshal(data, &legacy); err == nil {
		if legacy.ID == 0 && legacy.Title == "" && legacy.Message == "" {
			Messages = []Message{}
		} else {
			//旧版单条消息转化为消息信息数组
			Messages = []Message{legacy}
		}
		return nil
	}
	return fmt.Errorf("消息格式错误！")

}
func SaveMessage() error {
	//Message{} 是单条空消息。
	//Messages 是所有消息的切片。
	data, err := json.MarshalIndent(Messages, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(messageFilePath, data, 0644)

}

// 创建用户
func CreateUser(name string, password string) (*User, error) {
	if name == "" || password == "" {
		return nil, errors.New("用户名或密码不能为空！")
	}
	count := len(password)
	if count < 8 || count > 16 {
		return nil, errors.New("密码长度必须在8~16之间！")
	}
	count1 := 0
	count2 := 0
	for _, i := range password {
		if unicode.IsLower(i) {
			count1++
		}
		if unicode.IsUpper(i) {
			count2++
		}
	}
	if count1 == 0 || count2 == 0 {
		return nil, errors.New("密码里必须包含大小写字母！")
	}
	if (count - count1 - count2) == 0 {
		return nil, errors.New("密码里必须包含除了字母以外的其他符号，数字等等！")
	}
	if _, err := FindUserName(name); err == nil {
		return nil, errors.New("用户名已存在！")
	}
	newPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("密码加密失败")
		return nil, err
	}
	newID := int64(1)
	if len(Users) > 0 {
		newID = Users[len(Users)-1].ID + 1
	}
	user := &User{
		ID:       newID,
		Name:     name,
		Password: string(newPassword),
	}
	Users = append(Users, *user)
	err = SaveUser()
	if err != nil {
		return nil, errors.New("保存用户数据失败！")
	}
	return user, nil
}

// 检查用户对应信息
func CheckUser(username string, password string, u *User) (*User, error) {
	if username == "" || password == "" {
		return nil, errors.New("请输入用户名或密码！")
	}
	if u == nil {
		return nil, errors.New("用户不存在！")
	}
	if username != u.Name {
		return nil, errors.New("用户名错误或该用户不存在！")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, errors.New("密码错误！")
	}
	return u, nil
}

// 查看用户名是否存在
func FindUserName(name string) (*User, error) {
	for _, u := range Users {
		if u.Name == name {
			return &u, nil
		}
	}
	return nil, errors.New("用户不存在！")
}

// 登录
func LoginService(username, password string) (*User, error) {
	u, err := FindUserName(username)
	if err != nil {
		return nil, err
	}
	u2, err2 := CheckUser(username, password, u)
	if err2 != nil {
		return nil, err2
	}
	return u2, nil
}

// QQ消息
/*func CreateMessages(ctx context.Context, u *User) (*Message, error) {
	if u == nil {
		return nil, errors.New("用户不能为空")
	}

	qq, err := Get.NextMessage(ctx, 3*time.Second)
	if err != nil {
		return nil, err
	}

	newID := int64(1)
	if len(Messages) > 0 {
		newID = Messages[len(Messages)-1].ID + 1
	}

	message1 := &Message{
		ID:             newID,
		UserId:         u.ID,
		Title:          qq.Title,
		Message:        qq.Body,
		Time:           qq.Time,
		NotificationID: qq.NotificationID,
	}

	Messages = append(Messages, *message1)
	if err1 := SaveMessage(); err1 != nil {
		return nil, err1
	}
	return message1, nil
}*/

// 展示QQ消息内容
func ShowGetMessage(UserID int64, Id int) (string, error) {
	for _, i := range Messages {
		if i.UserId == UserID && i.ID == int64(Id) {
		}
		return limitRunes(i.Message, 5), nil
	}
	return "", errors.New("未找到匹配的消息")
}

// 展示QQ消息的标题
func ShowGetTitle(UserID int64, Id int) (string, error) {
	for _, i := range Messages {
		if i.UserId == UserID && i.ID == int64(Id) {
			return limitRunes(i.Title, 3), nil
		}
	}
	return "", errors.New("未找到匹配的消息")
}

// 获取桌面路径
func GetDesktopPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	desktop := filepath.Join(homeDir, "Desktop")
	//info.IsDir()判断当前文件信息是否代表一个目录（文件夹）
	if info, statErr := os.Stat(desktop); statErr == nil && info.IsDir() {
		return desktop, nil
	}
	oneDirveDesktop := filepath.Join(homeDir, "OneDirve", "Desktop")
	if info, statErr := os.Stat(oneDirveDesktop); statErr == nil && info.IsDir() {
		return oneDirveDesktop, nil
	}
	return desktop, nil
}

// 防止乱码
func limitRunes(value string, limit int) string {
	runes := []rune(value)
	//没超过限制返回原本字符串
	if len(runes) <= limit {
		return value
	}
	//截断
	return string(runes[:limit])
}
