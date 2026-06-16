package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

// run a bot
func runBot(bot *Bot, resume bool) {
	// logger
	log := logger.With(
		slog.Group("bot_info",
			slog.String("AppId", bot.AppId),
			slog.Bool("Sandbox", bot.Sandbox),
		),
	)
	bot.logger = log

	defer func() {
		if r := recover(); r != nil {
			log.Warn("Recovered in goroutine 'runBot'", "msg", r)
		}
	}()

	// startup
	if !bot.Enabled || bot.Webhook {
		return
	}
	log.Info("starting bot...", "resume", resume)
	bot.Status = STARTING

	// get token
	if !resume || bot.AK == nil || bot.WssUrl == "" {
		token, getTokenErr := retry.NewWithData[*oauth2.Token](
			retry.Attempts(5),
			retry.Delay(500*time.Millisecond),
		).Do(func() (*oauth2.Token, error) {
			return bot.Token()
		})
		if getTokenErr != nil {
			log.Error("Failed to retrieve the access token too many times.", "err", getTokenErr)
			bot.Status = STOPPED
			return
		}
		ts := oauth2.ReuseTokenSourceWithExpiry(token, bot, 5*time.Second)
		client := oauth2.NewClient(context.Background(), ts)
		bot.Client = client
		log.Info("Fetched access token.")

		// get wssUrl
		wssUrl, getWssErr := retry.NewWithData[string](
			retry.Attempts(5),
			retry.Delay(500*time.Millisecond),
		).Do(func() (string, error) {
			return bot.requestWssAddr()
		})
		if getWssErr != nil {
			log.Error("Failed to retrieve the WSS address too many times.", "err", getWssErr)
			bot.Status = STOPPED
			return
		}
		log.Info("Fetched WebSocket address.", "url", wssUrl)
		bot.WssUrl = wssUrl
	}

	header := http.Header{}
	header.Add("Authorization", bot.AK.TokenType+" "+bot.AK.AccessToken)
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	// establish connection
	conn, wssErr := retry.NewWithData[*websocket.Conn](
		retry.Attempts(5),
		retry.Delay(500*time.Millisecond),
	).Do(func() (*websocket.Conn, error) {
		conn, _, wssErr := dialer.Dial(bot.WssUrl, header)
		return conn, wssErr
	})
	if wssErr != nil {
		log.Error("Failed to establish connection too many times.", "err", wssErr)
		bot.Status = STOPPED
		return
	}
	bot.Connection = conn
	bot.WriteChannel = make(chan []byte, 10)
	bot.StopChannel = make(chan bool, 1)
	bot.Status = RUNNING

	// read HeartbeatInterval
	_, helloData, err := bot.Connection.ReadMessage()
	var first Payload
	var hello HelloPayload
	if err == nil {
		err = json.Unmarshal(helloData, &first)
		if err == nil {
			err = json.Unmarshal(first.Data, &hello)
		}
	}
	if err != nil {
		bot.Status = STOPPING
		log.Error("read from wss error.", "err", err)
		_ = bot.Connection.Close()
		bot.Status = STOPPED
		return
	}
	bot.HeartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond

	if resume {
		// resume
		err = conn.WriteMessage(websocket.TextMessage, []byte(
			fmt.Sprintf(`{"op":%d,"d":{"token":"%s %s","session_id":"%s","seq":%d}}`,
				Resume,
				bot.AK.TokenType,
				bot.AK.AccessToken,
				bot.SessionId,
				bot.Seq,
			),
		))
		if err != nil {
			bot.Status = STOPPING
			log.Error("write to wss error.", "err", err)
			_ = conn.Close()
			bot.Status = STOPPED
			return
		}
	} else {
		// authentication
		err = conn.WriteMessage(websocket.TextMessage, []byte(
			fmt.Sprintf(`{"op":%d,"d":{"token":"%s %s","intents":%d}}`, Identify, bot.AK.TokenType, bot.AK.AccessToken, bot.Intents),
		))
		if err != nil {
			bot.Status = STOPPING
			log.Error("write to wss error.", "err", err)
			_ = conn.Close()
			bot.Status = STOPPED
			return
		}

		// read session
		_, sessionData, err := conn.ReadMessage()
		if err != nil {
			bot.Status = STOPPING
			log.Error("read from error.", "err", err)
			_ = conn.Close()
			bot.Status = STOPPED
			return
		}
		var payload Payload
		var sessionPayload SessionPayload
		err = json.Unmarshal(sessionData, &payload)
		if err == nil {
			err = json.Unmarshal(payload.Data, &sessionPayload)
			bot.SessionId = sessionPayload.SessionId
		} else {
			bot.Status = STOPPING
			log.Error("read session fail.", "err", err)
			_ = conn.Close()
			bot.Status = STOPPED
			return
		}
	}

	// read message loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn("Recovered in goroutine 'read message'", "msg", r)
			}
		}()
		for {
			messageType, data, readErr := bot.Connection.ReadMessage()
			log.Debug("read from wss:", "messageType", messageType, "data", string(data))
			if readErr != nil {
				bot.Status = STOPPING
				log.Error("read from wss error.", "err", readErr)
				closeError, ok := errors.AsType[*websocket.CloseError](readErr)
				// may panic
				bot.StopChannel <- ok && closeError.Code == 4009
				return
			}
			if messageType != websocket.TextMessage {
				continue
			}
			go bot.handlerBotEvent(data)
		}
	}()

	// write message loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn("Recovered in goroutine 'write message'", "msg", r)
			}
		}()
		for {
			data := <-bot.WriteChannel
			writeErr := bot.Connection.WriteMessage(websocket.TextMessage, data)
			if writeErr != nil {
				bot.Status = STOPPING
				log.Error("write to wss err.", "err", writeErr)
				closeError, ok := errors.AsType[*websocket.CloseError](writeErr)
				// may panic
				bot.StopChannel <- ok && closeError.Code == 4009
				return
			}
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 判断是否满足发送心跳的条件
			if bot.Status == RUNNING && bot.LastHeartbeat.Add(bot.HeartbeatInterval-4*time.Second).Before(time.Now()) {
				bot.WriteChannel <- []byte(fmt.Sprintf("{\"op\":%d,\"d\":%d}", Heartbeat, bot.Seq))
				bot.LastHeartbeat = time.Now()
				log.Debug("send heartbeat", "AppId", bot.AppId)
			}
		case shouldResume := <-bot.StopChannel:
			// 接收到终止信号。该信号由读写协程或控制协程发出。
			close(bot.StopChannel)
			bot.Status = STOPPED
			bot.Connection = nil
			if shouldResume {
				go runBot(bot, shouldResume)
			}
			return
		}
	}

}

// maybe time-consuming
func (bot *Bot) handlerBotEvent(data []byte) {
	var payload Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		bot.logger.Error("unmarshal err:", "err", err)
		return
	}
	bot.Seq = payload.Seq
	switch payload.OPCode {
	case Dispatch:
		bot.dispatch(payload)
	}
}

func (bot *Bot) dispatch(payload Payload) {
	switch payload.Type {

	// c2c event
	case "C2C_MESSAGE_CREATE":
		var message ChatReceivedPayload
		if err := json.Unmarshal(payload.Data, &message); err != nil {
			bot.logger.Error("unmarshal err:", "err", err)
			return
		}
		bot.echo(message.Content, message.Id, message.Author.UserOpenid)
	// group event
	case "GROUP_AT_MESSAGE_CREATE":

	}
}

func (bot *Bot) echo(content string, msgId string, sender string) {
	err := bot.ReplyToMessage(content, msgId, sender)
	if err != nil {
		return
	}
}
