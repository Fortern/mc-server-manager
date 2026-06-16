package qq

import (
	"encoding/json"
	"time"
)

// ValidationPayload Validation Request
type ValidationPayload struct {
	PlainToken string `json:"plain_token"`
	EventTS    string `json:"event_ts"`
}

// AkPayload AccessToken Response
type AkPayload struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

type WsUrlPayload struct {
	Url string `json:"url"`
}

type HelloPayload struct {
	// HeartbeatInterval heartbeat interval in millisecond
	HeartbeatInterval int64 `json:"heartbeat_interval"`
}

type SessionPayload struct {
	Version   int    `json:"version"`
	SessionId string `json:"session_id"`
}

type ChatReceivedPayload struct {
	Id          string        `json:"id"`
	Author      MessageAuthor `json:"author"`
	Content     string        `json:"content"`
	MessageType int           `json:"message_type"`
}

type ChatSentPayload struct {
	Content   string `json:"content"`
	MsgType   int    `json:"msg_type"`
	Reference string `json:"message_reference,omitempty"`
	EventId   string `json:"event_id,omitempty"`
	MsgId     string `json:"msg_id,omitempty"`
}

type ChatConfirmPayload struct {
	Id        string `json:"id"`
	Timestamp time.Time
}

type MessageAuthor struct {
	IsBot       bool   `json:"bot"`
	Id          string `json:"id"`
	UnionOpenid string `json:"union_openid"`
	UserOpenid  string `json:"user_openid"`
	UserName    string `json:"user_name"`
}

type Payload struct {
	EventID string          `json:"id"`
	OPCode  OPCode          `json:"op"`
	Data    json.RawMessage `json:"d"`
	Seq     uint32          `json:"s"`
	Type    EventType       `json:"t"`
}

type OPCode int

type EventType string

const (
	Dispatch        OPCode = 0
	Heartbeat       OPCode = 1
	Identify        OPCode = 2
	Resume          OPCode = 6
	Reconnect       OPCode = 7
	InvalidSession  OPCode = 9
	Hello           OPCode = 10
	HeartbeatAck    OPCode = 11
	HttpCallbackAck OPCode = 12
	Validate        OPCode = 13
)
