package qq

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

type Bot struct {
	// AppId QQ `Bot` unique id
	AppId string `gorm:"primaryKey"`
	// AppSecret QQ `Bot` Secret
	AppSecret string
	// DeveloperQQ the developer's QQ number
	DeveloperQQ int `gorm:"column:developer_qq"`
	// Enabled If true, this bot will load when the project starts.
	Enabled bool
	// Webhook If true, use the web callback method for communication; otherwise, use WebSocket.
	Webhook bool
	// Sandbox If true, use the sandbox environment; otherwise, use the production environment.
	Sandbox bool
	// Intents represents a set of flag bits for the events subscribed to.
	Intents int

	CreatedAt time.Time
	UpdatedAt time.Time

	BotRuntime `gorm:"-:all"`
}

type BotRuntime struct {
	// Runtime Status
	Status Status
	// AK Oauth2 AccessToken
	AK *oauth2.Token
	// WssUrl
	WssUrl string
	// SessionId
	SessionId string
	// HeartbeatInterval heartbeat interval in millisecond
	HeartbeatInterval time.Duration
	// 上次发送心跳的时间
	LastHeartbeat time.Time
	// 最新序列号
	Seq uint32
	// Connection
	Connection *websocket.Conn
	// WriteChannel is used to notify the "write goroutine" to write data to the WebSocket connection
	WriteChannel chan []byte
	// 停止信号
	StopChannel chan bool
	// Client to request qq open api with stored token
	Client *http.Client
	// log
	logger *slog.Logger
}

type Status int

const (
	STOPPED Status = iota
	STARTING
	RUNNING
	RECONNECTING
	STOPPING
)
