package push

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"actor/third/log"
)

// var httpClient *rest.Client

type TgPusher struct {
	httpClient     *http.Client
	AlertBotToken  string
	AlertChatID    string
	AlertBaseURL   string
	AlertRobotId   string
	AlertRobotName string
	alertChan      chan _AlertPush
}

func NewTgPusher() *TgPusher {
	pusher := &TgPusher{}
	pusher.alertChan = make(chan _AlertPush, 100)

	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}
	pusher.httpClient = &http.Client{Transport: tr,
		Timeout: time.Second * 5,
	}

	go func() {
		for item := range pusher.alertChan {
			pusher.doPushTG(item.title, item.msg)
		}
	}()
	return pusher
}

// 查看chatID: 对话session 发/hi，然后在 https://api.telegram.org/bot{token}/getUpdates
// 要在log.Init之后调用
func (p *TgPusher) InitTG(botToken, chatID, robotId, robotName string) {
	// httpClient = rest.NewClient("", "", log.RootLogger)
	p.AlertBotToken = botToken
	p.AlertChatID = chatID
	p.AlertRobotId = robotId
	p.AlertRobotName = robotName
	p.AlertBaseURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
}

func (p *TgPusher) doPushTG(title, msg string) {
	params := url.Values{}
	params.Add("chat_id", p.AlertChatID)
	params.Add("parse_mode", "Markdown")
	params.Add("text", fmt.Sprintf("*%s*\nrobot:`%s`;`%s`\n`%s`", title, p.AlertRobotId, p.AlertRobotName, msg))
	{ // fasthttp
		// sc, err := httpClient.Request(http.MethodPost, AlertBaseURL, []byte(params.Encode()), map[string]string{
		// "Content-Type": "application/x-www-form-urlencoded",
		// }, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		// })
		// if err != nil || sc != 200 {
		// log.Errorf("发送 Telegram 消息失败: %v, status code:%d", err, sc)
		// return
		// }
	}
	{ // raw http

		request, err := http.NewRequest(http.MethodPost, AlertBaseURL, strings.NewReader(params.Encode()))
		if err != nil {
			log.Errorf("failed to NewRequest when send alert. %v\n", err)
			return
		}
		request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		response, err := httpClient.Do(request)
		if err != nil {
			log.Errorf("[utf_ign]failed to send alert. %v", err)
			return
		}
		response.Body.Close()
	}
}

func (p *TgPusher) PushAlert(title, msg string) {
	if p.AlertChatID == "" {
		log.Errorf("not inited pusher. want push: title %v, msg %v", title, msg)
		return
	}
	select {
	case p.alertChan <- _AlertPush{title: title, msg: msg}:
	default:
		log.Errorf("failed to insert alert msg cause full. %s, %s", title, msg)
	}
}

func (p *TgPusher) PushAlertCommon(msg string) {
	p.PushAlert("Common", msg)
}
