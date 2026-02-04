package actor

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"actor/broker/base"
	"actor/broker/base/generator"
	"actor/helper"
	"actor/helper/helper_ding"
	"actor/limit"
	lineswitcher "actor/line-switcher"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/fixed/fastfixed"
	"actor/third/log"
	bt "actor/tools"
	"go.uber.org/atomic"
)

var MuteStdout string // Y 表示 true

func init() {
	muteStdout := MuteStdout == "Y"
	if !base.IsUtf {
		if !muteStdout {
			fmt.Println(base.IsUtf)
		}
		if GitCommitHash == "" {
			fmt.Println(GitCommitHash)
			panic("请在构建时设置 actor.GitCommitHash")
		}
		if AppName == "" {
			panic("请在构建时设置 actor.AppName")
		}
		if BuildTime == "" {
			panic("请在构建时设置 actor.BuildTime")
		}
	} else {
		GitCommitHash = "the first test"
	}

	l := len(GitCommitHash)
	if l > 7 {
		GitCommitHash = GitCommitHash[0:7]
	}
	base.GitCommitHash = GitCommitHash
	base.AppName = AppName
	base.BuildTime = BuildTime
	// 避免循环依赖，多处设置
	helper.AppName = strings.ReplaceAll(AppName, "debugStra", "")
	helper.AppName = strings.ReplaceAll(helper.AppName, "beastStra", "")
	helper.BuildTime = helper.KeepOnlyNumbers(BuildTime)
	helper.GitCommitHash = GitCommitHash
	tsFmt := "YYYYMMDDHHmm"
	if len(helper.BuildTime) > len(tsFmt) {
		helper.BuildTime = helper.BuildTime[4:len(tsFmt)]
	}
	if !muteStdout {
		fmt.Println("bq commit: " + GitCommitHash)
	}
	log.Infof("bq commit: %s, app name %s, build time %s", GitCommitHash, AppName, BuildTime)
}

// GetWsClientAllow 获取exchange是否已经接入
func GetWsClientAllow(exchangeName string) bool {
	return generator.AllowWsClient(exchangeName)
}

const (
	ClientTypeRs int = 1 << iota
	ClientTypeWs int = 1 << iota
)

const ClientTypeAll = ClientTypeRs | ClientTypeWs

func redString(msg string) string {
	return fmt.Sprintf("\033[31m %s \033[0m", msg)
}

var goroCounterOnce = sync.Once{}

func GetClient(clientTypes int, exchangeName string, params helper.BrokerConfig, tradeMsg *helper.TradeMsg, cb helper.CallbackFunc) (base.Rs, base.Ws) {
	if params.Logger == nil {
		fmt.Fprintln(os.Stderr, redString("must provide logger in BrokerConfig"))
		params.RootLogger.Error("must provide logger in BrokerConfig")
		return nil, nil
	}
	if params.RootLogger == nil {
		fmt.Fprintln(os.Stderr, redString("must provide RootLogger in BrokerConfig"))
		params.RootLogger.Error("must provide RootLogger in BrokerConfig")
		return nil, nil
	}
	if params.RobotId == "" {
		fmt.Fprintln(os.Stderr, redString("must provide RobotId in BrokerConfig"))
		params.RootLogger.Error("must provide RobotId in BrokerConfig")
		return nil, nil
	}
	if params.NeedIndex && cb.OnIndex == nil {
		msg := "NeedIndex, but not provide OnIndex()"
		fmt.Fprintln(os.Stderr, redString(msg))
		params.RootLogger.Error(msg)
		return nil, nil
	}
	if params.NeedTrade && cb.OnTrade == nil {
		msg := "NeedTrade, but not provide OnTrade()"
		fmt.Fprintln(os.Stderr, redString(msg))
		params.RootLogger.Error(msg)
		return nil, nil
	}

	params.Logger.Infof("using params.Logger %p, params.RootLogger %p", params.Logger, params.RootLogger)
	params.RootLogger.Infof("using params.Logger %p, params.RootLogger %p", params.Logger, params.RootLogger)
	params.Name = exchangeName
	helper.RobotIdLite = strings.Split(params.RobotId, "-")[0]
	helper.RobotId = params.RobotId
	log.RootLogger = params.RootLogger
	// if !base.IsUtf {
	params2 := helper.BrokerConfigExt{}
	params2.BrokerConfig = params
	params2.Position = &helper.Pos{Pair: params.Pairs[0]}
	params2.PositionMap = cmap.New[*helper.Pos]()
	params2.EquityMapRs = cmap.New[*helper.EquityEvent]()
	params2.EquityMapWs = cmap.New[*helper.EquityEvent]()
	// }
	var rs base.Rs
	var ws base.Ws

	helper.InitAlerterSystemOwner(params.OwnerDeerKeys)

	//增加一层OrderEvent判断拦截
	if cb.OnOrder != nil {
		orig := cb.OnOrder
		// panic不会崩溃，用于调用处rsp handler打印出错的rsp报文
		cb.OnOrder = func(ts int64, event helper.OrderEvent) {
			// 仅在 new 的时候 强制要求 cid oid 同时存在且合法
			// 非 new 的时候 可能已经完成了 oid 的匹配 此时仅有 cid 或者 oid 依然能够正常完成推算
			if event.Type == helper.OrderEventTypeNEW {
				// adl 资费等订单 client id 为空，是正常情况，不需要block
				// if event.ClientID == "" {
				// helper.AlerterSystem.Push("wrong OrderEvent", "empty cid")
				// panic("empty cid")
				// }
				// 全部所cid都是字符串，不会出现"0"情况，只有oid为number类型转换时会出错
				// if event.ClientID == "0" {
				// helper.AlerterSystem.Push("wrong OrderEvent", "invalid cid")
				// panic("invalid cid")
				// }
				if event.OrderID == "" {
					helper.AlerterSystem.Push("wrong OrderEvent", "empty oid")
					panic("empty oid")
				}
				if event.OrderID == "0" {
					helper.AlerterSystem.Push("wrong OrderEvent", "invalid oid")
					panic("invalid oid")
				}
			}
			orig(ts, event)
		}
	}

	info := &helper.ExchangeInfo{}
	if clientTypes&ClientTypeRs > 0 {
		rs = getRsClient(exchangeName, params2, tradeMsg, info, cb)
		if rs == nil {
			params.Logger.Errorf("get rs client failed")
			return nil, nil
		}
		if !params.DisableLineSwitcher {
			params.Logger.Infof("enabled line switcher")
			lineswitcher.LineSwitcher.AddRs(rs)
		} else {
			params.Logger.Infof("disabled line switcher")
		}
	}
	if clientTypes&ClientTypeWs > 0 {
		origOnTicker := cb.OnTicker
		cb.OnTicker = func(tsns int64) {
			// 检测
			ws.UpdateLatestTickerTs(tsns)
			origOnTicker(tsns)
		}
		if cb.OnWsReconnecting != nil {
			lastNotifyTimeSec := time.Now().Unix()
			origOnWsReconnecting := cb.OnWsReconnecting
			cb.OnWsReconnecting = func(ex helper.BrokerName) {
				now := time.Now().Unix()
				if now-lastNotifyTimeSec > 60 {
					origOnWsReconnecting(ex)
					lastNotifyTimeSec = now
				} else {
					params.Logger.Warnf("too fast to call OnWsReconnecting, ignore this one")
				}
			}
		}
		if cb.OnTrade == nil {
			cb.OnTrade = func(ts int64) {}
		}
		ws = getWsClient(exchangeName, params2, tradeMsg, info, cb)
		if ws == nil {
			params.Logger.Errorf("get ws client failed")
			return nil, nil
		}
	}
	goroCounterOnce.Do(func() {
		if strings.Contains(strings.ToLower(base.AppName), "market") {
			return
		}
		go func() {
			threshold := 100
			if strings.Contains(strings.ToLower(base.AppName), "dino") {
				threshold = 100
			} else if strings.Contains(strings.ToLower(base.AppName), "ghost") {
				threshold = 500
			} else if strings.Contains(strings.ToLower(base.AppName), "omni") {
				threshold = 800
			} else if strings.Contains(strings.ToLower(base.AppName), "bunny") {
				threshold = 300
			} else if strings.Contains(strings.ToLower(base.AppName), "watch") {
				threshold = 500
			} else if strings.Contains(strings.ToLower(base.AppName), "market") {
				threshold = 5000
			} else if strings.Contains(strings.ToLower(base.AppName), "door") {
				threshold = 2000
			}
			last_num := runtime.NumGoroutine()
			for {
				gNum := runtime.NumGoroutine()
				if float64(gNum)/float64(last_num) > 1.2 && gNum > 100 {
					helper.AlerterSystemOwner.Push("线程1分钟增量>20%", fmt.Sprintf("gNum%d last_num%d", gNum, last_num))
					log.Infof("线程1分钟增量>20%", fmt.Sprintf("gNum%d last_num%d", gNum, last_num))
					logGoro(gNum)
				} else if gNum-last_num > 100 {
					helper.AlerterSystemOwner.Push("线程1分钟增量>100", fmt.Sprintf("gNum%d last_num%d", gNum, last_num))
					log.Infof("线程1分钟增量>100", fmt.Sprintf("gNum%d last_num%d", gNum, last_num))
					logGoro(gNum)
				} else if gNum > threshold && strings.Contains(strings.ToLower(base.AppName), "watch") {
					helper.AlerterSystemOwner.Push("运行线程数量异常", fmt.Sprintf("gNum%d", gNum))
					log.Infof("运行线程数量异常", fmt.Sprintf("gNum%d", gNum))
					logGoro(gNum)
				}
				last_num = gNum
				if gNum > threshold*3/2 {
					logGoro(gNum)
					log.Infof("onreset 运行线程数量异常 %v 请人工检查", gNum)
					cb.OnReset(fmt.Sprintf("运行线程数量异常 %v 请人工检查", gNum))
					return
				}
				time.Sleep(60 * time.Second)
			}
		}()
	})

	return rs, ws
}

func logGoro(gNum int) {
	buf := make([]byte, 1<<20) // 1 MB buffer
	stacklen := runtime.Stack(buf, true)
	aa := string(buf[:stacklen])
	helper.AlerterSystemOwner.Push("运行线程数量异常", fmt.Sprintf("运行线程数量异常 %v", gNum))
	log.Infof("[线程数量]%v", gNum)
	log.Infof("[线程堆栈]%v", aa)
}

// 根据盘口名称返回对应的接口
// 从外部传入 TradeMsg 交易数据集合 ExchangeInfo 交易对规则 的指针
// @param excludeIpsAsPossible nil时，使用文件里面的最快ip或者默认网络处理方式；非nil时，将会在尽量排除这些ip之后随机使用其他ip
func getWsClient(exchangeName string, params helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	params.ActivateDelayMonitor = false
	if msg == nil {
		helper.LogErrorThenCall(fmt.Sprintf("trade msg cannot empty "+exchangeName), cb.OnExit)
		return nil
	}
	exchangeName = strings.TrimSpace(exchangeName)
	// pair := &params.Pair
	// pair.Base = strings.TrimSpace(pair.Base)
	// pair.Base = helper.Trim10Multiple(pair.Base)
	// pair.Quote = strings.TrimSpace(pair.Quote)
	// pair.More = strings.TrimSpace(pair.More)
	if cb.OnIndex == nil {
		cb.OnIndex = func(ts int64, index helper.IndexEvent) {}
	}
	if params.Logger == nil {
		if log.RootLogger == nil {
			panic("must init logger before call GetRsClient")
		}
		params.Logger = log.RootLogger
	}
	params.Logger = log.WrapLogger{Logger: params.Logger}
	params.Logger.Infof("bq commit: %s", GitCommitHash)
	params.Logger.Infof("GetWsClient BrokerConfig %s %v", exchangeName, params.StringLite())
	ws := generator.GenerateWs(exchangeName, &params, msg, info, cb)
	if ws == nil {
		if cb.OnExit != nil {
			cb.OnExit(fmt.Sprintf("未找到该盘口: " + exchangeName))
			return nil
		}
	}
	// ws.SetPairs(params.Pairs)
	// ws.SetAcctMsgMap(params.AcctMsgMap)
	if params.AlertCarryInfo != "" {
		helper_ding.AlertCarryInfo = params.AlertCarryInfo
	}
	ip := limit.GetMyIP()
	if len(ip) == 0 {
		helper.LogErrorThenCall(fmt.Sprintf("无法获取本机ip"+exchangeName), cb.OnExit)
		return nil
	}
	id := bt.MustConvertIpToId(ip[0])
	msg.Ticker.Seq.InnerServerId = id
	// msg.Equity.Seq.InnerServerId = id
	// msg.Position.Seq.InnerServerId = id
	if params.AlertCarryInfo != "" {
		helper_ding.AlertCarryInfo = params.AlertCarryInfo
	}
	return ws
}

// GetRsClientAllow 获取exchange是否已经接入
func GetRsClientAllow(exchangeName string) bool {
	return generator.AllowRsClient(exchangeName)
}

func GetAllowExchanges() []string {
	return generator.GetAllowExchanges()
}

// 根据盘口名称返回对应的接口
func getRsClient(exchangeName string, params helper.BrokerConfigExt, msg *helper.TradeMsg, info *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if msg == nil {
		helper.LogErrorThenCall(fmt.Sprintf("trade msg cannot empty "+exchangeName), cb.OnExit)
		return nil
	}
	exchangeName = strings.TrimSpace(exchangeName)
	// pair := &params.Pair
	// pair.Base = strings.TrimSpace(pair.Base)
	// pair.Base = helper.Trim10Multiple(pair.Base)
	// pair.Quote = strings.TrimSpace(pair.Quote)
	// pair.More = strings.TrimSpace(pair.More)
	if cb.OnIndex == nil {
		cb.OnIndex = func(ts int64, index helper.IndexEvent) {}
	}
	if params.Logger == nil {
		if log.RootLogger == nil {
			panic("must init logger before call GetRsClient")
		}
		params.Logger = log.RootLogger
	}
	params.Logger = log.WrapLogger{Logger: params.Logger}
	params.Logger.Infof("bq commit: %s", GitCommitHash)
	params.Logger.Infof("GetRsClient BrokerConfig %s %v", exchangeName, params.StringLite())
	rs := generator.GenerateRs(exchangeName, &params, msg, info, cb)
	if rs == nil {
		if cb.OnExit != nil {
			cb.OnExit(fmt.Sprintf("未找到该盘口: " + exchangeName))
		}
		return rs
	}
	if params.AlertCarryInfo != "" {
		helper_ding.AlertCarryInfo = params.AlertCarryInfo
	}

	ip := limit.GetMyIP()
	if len(ip) == 0 {
		helper.LogErrorThenCall(fmt.Sprintf("无法获取本机ip"+exchangeName), cb.OnExit)
		return nil
	}
	id := bt.MustConvertIpToId(ip[0])
	msg.Ticker.Seq.InnerServerId = id
	// msg.Equity.Seq.InnerServerId = id
	// msg.Position.Seq.InnerServerId = id

	return rs
}

func SetDingTokens(normal, serious string) {
	helper_ding.SetDingTokens(normal, serious)
}

// 参数exPairs 格式为 key: exchange name, value: pairs
var detected atomic.Bool

func DetectPrecision(brokerConfigs []helper.BrokerConfig) error {
	if detected.CompareAndSwap(false, true) {
		cb := helper.CallbackFunc{
			OnExit: func(msg string) {
				log.Errorf("detect precision failed, OnExit %s", msg)
				panic(msg)
			},
			OnReset: func(msg string) {
			},
			OnTicker: func(ts int64) {},
			OnMsg:    func(msg string) {},
			OnDetail: func(msg string) {},
		}
		for _, brokerConfig := range brokerConfigs {
			brokerConfigTmp := brokerConfig
			brokerConfigTmp.NeedAuth = false
			brokerConfigTmp.NeedTicker = false
			brokerConfigTmp.WsDepthLevel = 0
			brokerConfigTmp.NeedTrade = false
			brokerConfigTmp.NeedIndex = false
			brokerConfigTmp.NeedMarketLiquidation = false

			rsClient, _ := GetClient(ClientTypeRs, brokerConfig.Name, brokerConfigTmp, nil, cb)
			if rsClient == nil {
				return fmt.Errorf("detect precision failed %s", brokerConfig.Name)
			}
			rsClient.BeforeTrade(helper.HandleModePublic)
			// rsClient.GetExchangeInfos()
			for _, pair := range brokerConfig.Pairs {
				pairinfo, ok := rsClient.GetPairInfoByPair(&pair)
				if !ok {
					msg := fmt.Sprintf("not found pairinfo %s in %s", pair.String(), brokerConfig.Name)
					log.Errorf(msg)
					return fmt.Errorf("%s", msg)
				}
				if pairinfo.TickSize < (1.0 / fastfixed.NPlaces) {
					msg := fmt.Sprintf("will use slow high decimal, because tick size %f is too small in %s@%s", pairinfo.TickSize, brokerConfig.Name, pair.String())
					log.Warnf(msg)
					fixed.TurnOnHighDecimal()
					return nil
				}
				if pairinfo.StepSize < (1.0 / fastfixed.NPlaces) {
					msg := fmt.Sprintf("will use slow high decimal, because step size %f is too small in %s@%s", pairinfo.StepSize, brokerConfig.Name, pair.String())
					log.Warnf(msg)
					fixed.TurnOnHighDecimal()
					return nil
				}
			}
		}
		log.Infof("will use fast decimal")
		fixed.TurnOnFastDecimal()
	} else {
		return fmt.Errorf("precision already detected")
	}
	return nil
}

var GitCommitHash string = ""
var AppName string = ""   // 上层应用的名字, 例如 debugStraXXX
var BuildTime string = "" // 构建时间
