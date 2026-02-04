package quant

import (
	"actor"
	"actor/helper"
	"actor/robot/cat/config"
	"actor/third/log"
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

// QuantInitConfig 调度器初始化函数
func (q *Quant) QuantInitConfig(logFile string) (err error) {
	q.cfg = config.Session()
	q.logger = log.InitWithLogger(
		logFile,
		q.cfg.LogLevel,
		log.SetStdout(false),
		log.SetCaller(true),
		log.SetMaxBackups(30),
	)
	q.elog = logrus.WithField("rid", q.cfg.TaskUid)

	// 获取本机私有ip和公网id对应表
	ips, err := helper.GetClientIp()
	if err != nil {
		panic("获取本机ip失败")
	}
	q.StatusLock.Lock()
	q.Status = helper.TaskStatusPrepare
	q.StatusLock.Unlock()

	// 找到对应的公网ip
	pubIpList, priIpMap := helper.GetIpPool(ips)
	log.Infof("IP POOL: %v PUB IP List: %v", priIpMap, pubIpList)
	log.Infof("公网ip数量:%v 私有ip数量:%v", len(pubIpList), len(priIpMap))

	if len(priIpMap) == 0 {
		panic("获取本机ip池失败")
	} else if len(priIpMap) != len(ips) {
		log.Infof("获取本机ip池可能失败")
	} else {
		log.Infof("可用IP数量: %v", len(priIpMap))
	}

	// trade ip 转换
	if q.cfg.TradeIP == "" {
		log.Infof("使用默认网卡默认发包ip")
	} else {
		if _, ok := priIpMap[q.cfg.TradeIP]; ok {
			log.Warnf("配置文件传入的是私有ip %v 无需转换直接用于发包", q.cfg.TradeIP)
		} else {
			find := false
			for priIp, pubIp := range priIpMap {
				log.Debugf("搜索ip 私有%v 公网%v 交易%v", priIp, pubIp, q.cfg.TradeIP)
				if q.cfg.TradeIP == pubIp || strings.Contains(q.cfg.TradeIP, pubIp) {
					find = true
					q.cfg.TradeIP = priIp
					log.Warnf("tradeIp为公网ip 转换为私有ip:%v 用于交易", q.cfg.TradeIP)
					break
				}
			}
			if !find {
				q.StatusLock.Lock()
				q.Status = helper.TaskStatusError
				q.StatusLock.Unlock()
				return fmt.Errorf("trade_ip 异常 本托管者不支持:%v", q.cfg.TradeIP)
			}
		}
	}

	q.tradeMsgLocal = NewTradeMsgLocal()

	// 初始化 restApi 信息
	q.stoploss = q.cfg.StrategyParams.StopLoss
	q.tradePair, _ = helper.StringPairToPair(q.cfg.StrategyParams.Pair)
	q.tradeExchangeName = q.cfg.Exchange
	q.tradeAcctName = q.cfg.AcctName

	q.tradeRest, _ = actor.GetClient(
		actor.ClientTypeRs,
		q.tradeExchangeName,
		helper.BrokerConfig{
			Name:          q.tradeAcctName,
			AccessKey:     q.cfg.AccessKey,
			SecretKey:     q.cfg.SecretKey,
			PassKey:       q.cfg.PassKey,
			Pairs:         []helper.Pair{q.tradePair},
			ProxyURL:      q.cfg.Proxy,
			SymbolAll:     true,
			NeedAuth:      true,
			LocalAddr:     q.cfg.TradeIP,
			Logger:        q.logger,
			RootLogger:    log.RootLogger,
			RobotId:       q.cfg.TaskUid,
			OwnerDeerKeys: []string{},
		},
		&q.tradeTradeMsg,
		helper.CallbackFunc{
			OnTicker:        q.onTicker,
			OnEquityEvent:   q.onEquity,
			OnPositionEvent: q.onPosition,
			OnOrder:         q.onOrder,
			OnDepth:         func(ts int64) {},
			OnReset:         q.onReset,
			OnExit:          q.OnExit,
		},
	)
	q.tradePairInfo = *q.tradeRest.GetPairInfo()

	// 获取交易规则
	q.infos = q.tradeRest.GetExchangeInfos()
	q.ExchangeInfoS2P = make(map[string]helper.ExchangeInfo)
	for _, info := range q.infos {
		q.ExchangeInfoS2P[info.Symbol] = info
		if info.Pair.Base == q.tradePair.Base {
			q.tradePairInfo = info
		}
	}

	q.tradeRest.BeforeTrade(helper.HandleModePrepare)

	// 获取账户资金
	tradeCash, ok := q.tradeRest.ObtainEquityMap().Get("usdt")
	if ok {
		q.tradeMsgLocal.EquityEvent = tradeCash
		q.totalCash = q.tradeMsgLocal.EquityEvent.Avail
	}

	totalCashTrade := q.totalCash
	if totalCashTrade < 5 {
		q.StatusLock.Lock()
		q.Status = helper.TaskStatusError
		q.StatusLock.Unlock()
		return fmt.Errorf("交易盘口 总资金不足5u")
	}

	if err = q.SetAccountMode(); err != nil {
		return err
	}

	q.ResetCash()

	log.Infof("止损点:%.4f", q.cfg.StrategyParams.StopLoss)

	// 初始化K线
	q.ResetCandles()

	// 创建ws
	_, q.tradeWs = actor.GetClient(
		actor.ClientTypeWs,
		q.tradeExchangeName,
		helper.BrokerConfig{
			Name:          q.tradeAcctName,
			AccessKey:     q.cfg.AccessKey,
			SecretKey:     q.cfg.SecretKey,
			PassKey:       q.cfg.PassKey,
			Pairs:         []helper.Pair{q.tradePair},
			SymbolAll:     true,
			ProxyURL:      q.cfg.Proxy,
			NeedAuth:      true,
			NeedTicker:    true,
			NeedTrade:     false,
			LocalAddr:     q.cfg.TradeIP,
			RootLogger:    log.RootLogger,
			Logger:        q.logger,
			RobotId:       q.cfg.TaskUid,
			OwnerDeerKeys: []string{},
			Need1MinKline: true,
		},
		&q.tradeTradeMsg,
		helper.CallbackFunc{
			OnTicker:        q.onTicker,
			OnOrder:         q.onOrder,
			OnEquityEvent:   q.onEquity,
			OnPositionEvent: q.onPosition,
			OnDepth:         q.onDepth,
			OnTrade:         q.onTrade,
			OnReset:         q.onReset,
			OnExit:          q.OnExit,
			OnKline:         q.OnKline,
		})

	q.tradeWs.Run()
	q.isTradeSpot = strings.Contains(q.tradeExchangeName, "spot")
	log.Infof("判断一下盘口性质 isTradeSpot:%v", q.isTradeSpot)
	log.Infof("=======策略配置======初始化成功")
	return nil
}

// NewQuant 创建新的 Quant 实例
func NewQuant(ctx context.Context, buildTime string) *Quant {
	childCtx, cancel := context.WithCancel(ctx)
	q := &Quant{
		ctx:       childCtx,
		cancel:    cancel,
		Status:    helper.TaskStatusPrepare,
		buildTime: buildTime,
	}
	return q
}
