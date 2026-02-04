package helper

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"actor/third/log"
)

const logUrl = ""

// ChannelEnum 告警渠道
type ChannelEnum int

var RobotIdLite string
var RobotId string

const (
	ChannelHFT       ChannelEnum = iota + 1 // 高频策略报警
	ChannelArbitrage                        // 套利策略报警
	ChannelInterface                        // 接口组报警
	ChannelFullStack                        // 全栈组报警
	ChannelOther                            // 其他报警
	ChannelMid                              // 中频报警
)

// AlertLevel 告警级别
type AlertLevel int

const (
	AlertLevelFatal AlertLevel = iota + 1
	AlertLevelError
	AlertLevelWarn
	AlertLevelInfo
	AlertLevelDebug
)

type AlertConfig struct {
	Channel  ChannelEnum `json:"channel"`
	DeerKeys []string    `json:"deer_keys"`
}

type Alerter struct {
	AlertConfig
	msgChan chan _AlertMsg
	client  *http.Client
}

func (a *Alerter) Stop() {
	CloseSafe(a.msgChan)
}

type _AlertMsg struct {
	Title string
	Msg   string
	Level AlertLevel
}

type AlertParams struct {
	Title      string      `json:"title"`
	Message    string      `json:"message"`
	AlertLevel AlertLevel  `json:"alertLevel"`
	Channel    ChannelEnum `json:"channel"`
	DeerKeys   []string    `json:"deer_keys"`
	RobotID    string      `json:"robotID"`
}

func init() {
	initAlerterSystem(nil)
}

var AlerterSystem *Alerter
var AlerterSystemOwner *Alerter
var onceInitAlerterSystemOwner sync.Once

func initAlerterSystem(k []string) {
	cfg := AlertConfig{}
	cfg.Channel = ChannelInterface
	//==========todo=======================
	var err error
	AlerterSystem, err = NewAlert(cfg)
	if err != nil {
		panic(err)
	}
}
func InitAlerterSystemOwner(k []string) {
	onceInitAlerterSystemOwner.Do(func() {
		cfg := AlertConfig{}
		cfg.Channel = ChannelInterface
		keys := make(map[string]string)
		for _, k0 := range k {
			keys[k0] = ""
		}
		cfg.DeerKeys = make([]string, 0)
		for k0 := range keys {
			cfg.DeerKeys = append(cfg.DeerKeys, k0)
		}
		var err error
		AlerterSystemOwner, err = NewAlert(cfg)
		if err != nil {
			panic(err)
		}
	})
}

func (a *Alerter) Fatal(message string) {
	a.send(message, AlertLevelFatal)
}

func (a *Alerter) Error(message string) {
	a.send(message, AlertLevelError)
}

func (a *Alerter) Push(title, message string) {
	a.sendWithTitle(title, message, AlertLevelError)
}

func (a *Alerter) Warn(message string) {
	a.send(message, AlertLevelWarn)
}

func (a *Alerter) Info(message string) {
	a.send(message, AlertLevelInfo)
}

func (a *Alerter) Debug(message string) {
	a.send(message, AlertLevelDebug)
}
func (a *Alerter) send(message string, level AlertLevel) {
	select {
	case a.msgChan <- _AlertMsg{Msg: message, Level: level}:
	default:
		log.Errorf("failed to send alert to chan")
	}
}
func (a *Alerter) sendWithTitle(title, message string, level AlertLevel) {
	select {
	case a.msgChan <- _AlertMsg{Title: title, Msg: message, Level: level}:
	default:
		log.Errorf("failed to send alert to chan")
	}
}

func (a *Alerter) log(params AlertParams) {

}

// 不用时调用 Stop()，释放资源
func NewAlert(params AlertConfig) (*Alerter, error) {
	if params.Channel == 0 {
		return nil, errors.New("初始化log失败，channel不能为空")
	}
	alert := &Alerter{}
	alert.AlertConfig = params
	alert.msgChan = make(chan _AlertMsg, 100)
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}
	alert.client = &http.Client{
		Transport: tr,
		Timeout:   time.Second * 5,
	}
	go func(alert *Alerter) {
		for {
			select {
			case item, ok := <-alert.msgChan:
				if !ok {
					return
				}
				msg := AlertParams{
					Title:      item.Title,
					Message:    fmt.Sprintf("[rid:%s;app:%s;bt:%s;bq:%s]\n%s", RobotIdLite, AppName, BuildTime, GitCommitHash, item.Msg),
					AlertLevel: item.Level,
					Channel:    params.Channel,
					RobotID:    RobotId,
					DeerKeys:   params.DeerKeys,
				}
				alert.log(msg)
			}
		}
	}(alert)
	return alert, nil
}
