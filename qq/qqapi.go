package qq

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

var (
	db       *gorm.DB
	botCache *freecache.Cache
	botMap   *sync.Map
)

const (
	TokenType = "QQBot"
	// HeaderTraceID 机器人openapi返回的链路追踪ID
	HeaderTraceID = "X-Tps-trace-ID"
	// APIDomain api domain
	APIDomain = "https://api.sgroup.qq.com"
	// SandBoxAPIDomain sandbox domain
	SandBoxAPIDomain = "https://sandbox.api.sgroup.qq.com"
	// GatewayPath to get wss url
	GatewayPath = "/gateway"
	// UserMessagePath to send message to a user
	UserMessagePath = "/v2/users/%s/messages"
	// GetTokenUrl to get AccessToken
	GetTokenUrl = "https://bots.qq.com/app/getAppAccessToken"
)

// MaxIdleConns 默认指定空闲连接池大小
const MaxIdleConns = 3000

// TODO 删除
var tmpBots = map[string]*Bot{
	"1904155783": {
		AppId:       "1904155783",
		AppSecret:   "qcCYhdMr8C2f4FCv",
		DeveloperQQ: 1007305659,
		Enabled:     true,
	},
}

// GetBotById 根据ID获取机器人
func GetBotById(id string) *Bot {
	value, err := botCache.Get([]byte(id))
	if err == nil {
		return deserialize(value)
	}
	slog.Error("get bot from cache err:", "err", err)
	bot, err := gorm.G[Bot](db).Where("AppId = ?", id).First(context.Background())
	if err != nil {
		slog.Error("get bot from db err:", "err", err)
	}
	return &bot
}

// UpdateBotById TODO 修改机器人信息
func UpdateBotById(id string) {

}

// InitBot 初始化Bot TODO 从数据库中读取数据
func InitBot(botDb *gorm.DB) {
	db = botDb

	ctx := context.Background()
	bots, err := gorm.G[Bot](db).Where("enabled = ?", true).Find(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("InitBot err.", "err", err)
		}
		return
	}

	botCache = freecache.NewCache(512 * 1024)
	botMap = &sync.Map{}

	for _, bot := range bots {
		botMap.Store(bot.AppId, &bot)
		if bot.Webhook {
			continue
		}
		go runBot(&bot, false)
	}

}

func serialize(bot *Bot) []byte {
	// TODO serialize
	return nil
}

func deserialize(data []byte) *Bot {
	// TODO deserialize
	return nil
}

func (bot *Bot) requestWssAddr() (string, error) {
	var url string
	if bot.Sandbox {
		url = SandBoxAPIDomain + GatewayPath
	} else {
		url = APIDomain + GatewayPath
	}
	resp, err := bot.Client.Get(url)
	if err != nil {
		return "", err
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	code := resp.StatusCode
	if code < 200 || code > 299 {
		return "", fmt.Errorf("response code: %d, response body: %s", code, respBody)
	}
	var urlBody WsUrlPayload
	err = json.Unmarshal(respBody, &urlBody)
	if err != nil {
		return "", err
	}
	return urlBody.Url, nil
}

func (bot *Bot) Token() (*oauth2.Token, error) {
	resp, err := http.Post(
		GetTokenUrl,
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"appId":"%s","clientSecret":"%s"}`, bot.AppId, bot.AppSecret)),
	)
	if err != nil {
		return nil, err
	}
	code := resp.StatusCode
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("response code: %d, response body: %s", code, respBody)
	}

	var akBody AkPayload
	err = json.Unmarshal(respBody, &akBody)
	if err != nil {
		err0 := fmt.Errorf("response body: %s\n%w", string(respBody), err)
		return nil, err0
	}
	expiresIn, err := strconv.Atoi(akBody.ExpiresIn)
	if err != nil {
		err0 := fmt.Errorf("Unable to parse response body: %s\n%w", string(respBody), err)
		return nil, err0
	}
	token := &oauth2.Token{
		AccessToken: akBody.AccessToken,
		TokenType:   TokenType,
		ExpiresIn:   int64(expiresIn),
		Expiry:      time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	bot.AK = token
	return token, nil
}

func (bot *Bot) Sign(validationPayload ValidationPayload) (string, error) {
	seed := bot.AppSecret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	seed = seed[:ed25519.SeedSize]
	reader := strings.NewReader(seed)
	_, privateKey, err := ed25519.GenerateKey(reader)
	if err != nil {
		return "", err
	}
	var msg bytes.Buffer
	msg.WriteString(validationPayload.EventTS)
	msg.WriteString(validationPayload.PlainToken)
	signature := hex.EncodeToString(ed25519.Sign(privateKey, msg.Bytes()))
	return signature, nil
}

func (bot *Bot) ReplyToMessage(content string, msgId string, sender string) error {
	var url string
	if bot.Sandbox {
		url = SandBoxAPIDomain + fmt.Sprintf(UserMessagePath, sender)
	} else {
		url = APIDomain + fmt.Sprintf(UserMessagePath, sender)
	}
	msgBody := &ChatSentPayload{
		Content: content,
		MsgId:   msgId,
		MsgType: 0,
	}
	requestBody, err := json.Marshal(msgBody)
	if err != nil {
		return err
	}

	resp, err := bot.Client.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	var msgRespBody ChatConfirmPayload
	err = json.Unmarshal(respBody, &msgRespBody)
	if err != nil {
		return err
	}
	return nil
}
