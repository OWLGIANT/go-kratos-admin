package base

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/slice"
	client "github.com/influxdata/influxdb1-client/v2"

	"actor/broker/brokerconfig"
	"actor/config"
	"actor/helper"
	"actor/helper/local"
	"actor/push"
	"actor/third/cmap"
	"actor/third/fixed"
	cqueue "actor/third/gcircularqueue_generic"
	"actor/third/log"
	"actor/tools"
)

const RB_SIZE = 64
const RESET_DELAY_SEC = 5 * 60

var QUERY_DELAY_SEC = 60
var SEND_DELAY_SEC = 60

type EndpointKey_T int

func (e EndpointKey_T) String() string {
	switch e {
	case EndpointKey_Nil:
		return "Nil"
	case EndpointKey_RsNorPlace:
		return "RsNorPlace"
	case EndpointKey_RsColoPlace:
		return "RsColoPlace"
	case EndpointKey_RsNorCancel:
		return "RsNorCancel"
	case EndpointKey_RsColoCancel:
		return "RsColoCancel"
	case EndpointKey_RsNorAmend:
		return "RsNorAmend"
	case EndpointKey_RsColoAmend:
		return "RsColoAmend"
		// ws:
	case EndpointKey_WsNorPlace:
		return "WsNorPlace"
	case EndpointKey_WsColoPlace:
		return "WsColoPlace"
	case EndpointKey_WsNorCancel:
		return "WsNorCancel"
	case EndpointKey_WsColoCancel:
		return "WsColoCancel"
	case EndpointKey_WsNorAmend:
		return "WsNorAmend"
	case EndpointKey_WsColoAmend:
		return "WsColoAmend"
	//
	case EndpointKey_EndBarrier:
		return "EndBarrier"
	}
	i := int(e)
	return strconv.Itoa(i)
}
func (e EndpointKey_T) GetFactors() (client ClientType, link LinkType, action ActionType) {
	switch e {
	case EndpointKey_RsNorPlace:
		client = ClientType_Rs
		link = LinkType_Nor
		action = ActionType_Place
	case EndpointKey_RsColoPlace:
		client = ClientType_Rs
		link = LinkType_Colo
		action = ActionType_Place
	case EndpointKey_RsNorCancel:
		client = ClientType_Rs
		link = LinkType_Nor
		action = ActionType_Cancel
	case EndpointKey_RsColoCancel:
		client = ClientType_Rs
		link = LinkType_Colo
		action = ActionType_Cancel
	case EndpointKey_RsNorAmend:
		link = LinkType_Nor
		client = ClientType_Rs
		action = ActionType_Amend
	case EndpointKey_RsColoAmend:
		client = ClientType_Rs
		link = LinkType_Colo
		action = ActionType_Amend
	// case // ws:
	case EndpointKey_WsNorPlace:
		client = ClientType_Ws
		link = LinkType_Nor
		action = ActionType_Place
	case EndpointKey_WsColoPlace:
		client = ClientType_Ws
		link = LinkType_Colo
		action = ActionType_Place
	case EndpointKey_WsNorCancel:
		client = ClientType_Ws
		link = LinkType_Nor
		action = ActionType_Cancel
	case EndpointKey_WsColoCancel:
		client = ClientType_Ws
		link = LinkType_Colo
		action = ActionType_Cancel
	case EndpointKey_WsNorAmend:
		client = ClientType_Ws
		link = LinkType_Nor
		action = ActionType_Amend
	case EndpointKey_WsColoAmend:
		client = ClientType_Ws
		link = LinkType_Colo
		action = ActionType_Amend
	default:
		panic(fmt.Sprintf("not support key. %d", int(e)))
	}
	return
}

const (
	EndpointKey_Nil EndpointKey_T = iota
	EndpointKey_RsNorPlace
	EndpointKey_RsColoPlace
	EndpointKey_RsNorCancel
	EndpointKey_RsColoCancel
	EndpointKey_RsNorAmend
	EndpointKey_RsColoAmend
	// ws
	EndpointKey_WsNorPlace
	EndpointKey_WsColoPlace
	EndpointKey_WsNorCancel
	EndpointKey_WsColoCancel
	EndpointKey_WsNorAmend
	EndpointKey_WsColoAmend
	//
	EndpointKey_EndBarrier // 特殊交易所可以用 EndpointKey_EndBarrier + n 自定义
)

const monitorOrderCidPrefix = "M_"

type MonitorConfiguration struct {
	ip          string
	area        string
	exArea      string
	Zone        string
	exName      string
	pair        string
	crossDomain bool
}

type LineAction struct {
	Line          Line
	ActionWithExt string // like place,amend_pd
}

func (l LineAction) String() string {
	return fmt.Sprintf("%s_%s", l.Line.String(), l.ActionWithExt)
}

type DelayMonitor struct {
	MonitorConfiguration       // 放置一些静态信息
	monitoring                 atomic.Bool
	frs                        *FatherRs
	lines                      []Line
	monitorId                  string
	endPoints                  cmap.ConcurrentMap[EndpointKey_T, *Endpoint]
	entries                    cmap.ConcurrentMap[LineAction, *Endpoint]
	stopChan                   chan struct{}
	monitorTrigger             func(key EndpointKey_T, activeFirst bool) // activeFirst monitor发出的第一次下单请求，用于激活链路，ws不用激活
	influxClient               client.Client
	cidPrefix                  string
	cidPrefixLen               int
	mockCidForActive           string
	curOid                     string // 挂单的oid
	orderActionRspChan         chan MonitorOrderActionRsp
	alerter                    *helper.Alerter
	callExitIfSwitchLineFailed bool
	notSupportSwitchLine       bool
}

func (m *DelayMonitor) Tags() map[string]string {
	tags := make(map[string]string, 0)
	tags["ip"] = m.ip
	tags["area"] = m.area
	tags["ex_area"] = m.exArea
	tags["zone"] = m.Zone
	tags["ex"] = m.exName
	tags["pair"] = m.pair
	if m.crossDomain {
		tags["cross_domain"] = "true"
	} else {
		tags["cross_domain"] = "false"
	}
	return tags
}

type InfluxDBConfig struct {
	Addr string // ip or ip:port
	User string
	Pw   string
}

func DefaultInfluxConfig(params *helper.BrokerConfigExt) InfluxDBConfig {
	hostWithPort := config.INFLUX_ADDR
	if !strings.Contains(hostWithPort, ":") {
		hostWithPort += ":8086"
	}
	return InfluxDBConfig{
		Addr: hostWithPort,
		User: config.INFLUX_USER,
		Pw:   config.INFLUX_PWD,
	}
}

type LinkType string

const (
	LinkType_Nil  LinkType = ""
	LinkType_Colo LinkType = "colo"
	LinkType_Nor  LinkType = "nor"
)

// client,link 的元类型可以称为 LinePoint

type ClientType string

const (
	ClientType_Nil ClientType = ""
	ClientType_Rs  ClientType = "rs"
	ClientType_Ws  ClientType = "ws"
)

type SelectedLine struct {
	Weight int
	Line
}

func (s *SelectedLine) String() string {
	return fmt.Sprintf("(Weight %v, Line %v)", s.Weight, s.Line)
}

func (s *SelectedLine) Equal(s0 *SelectedLine) bool {
	return s.Client == s0.Client && s.Link == s0.Link
}

// compare and store, return true if changed
func (s *SelectedLine) CAS(s0 *SelectedLine) bool {
	if s.Client == s0.Client && s.Link == s0.Link {
		return false
	}
	s.Client = s0.Client
	s.Link = s0.Link
	return true
}

type Line struct {
	Client     ClientType
	Link       LinkType
	MarginMode helper.MarginMode
}

func (l Line) String() string {
	return fmt.Sprintf("%s-%s-%s", l.Client, l.Link, l.MarginMode)
}

type MonitorOrderActionRsp struct {
	Client     ClientType
	Action     ActionType
	Cid        string
	Oid        string // place action 必须填入oid
	AmendSucc  bool
	DurationUs int64
}

// monitorOrderPlacer 流程：cancel一个不存在的订单激活链路；place; amend; cancel
func InitDelayMonitor(mon *DelayMonitor, f *FatherRs, influxCfg InfluxDBConfig, exName string, pair string, monitorTrigger func(key EndpointKey_T, activeFirstDeprecated bool)) {
	if f.BrokerConfig.AccountName == "" {
		panic("BrokerConfig.AccountName should not empty")
	}
	mon.callExitIfSwitchLineFailed = f.BrokerConfig.CallExitIfSwitchLineFailed
	if IsUtf {
		QUERY_DELAY_SEC = 5
		SEND_DELAY_SEC = 10
	}

	mon.frs = f
	if mon.lines == nil { // 可能已经在AddLine中初始化
		mon.lines = make([]Line, 0)
	}
	ips, _ := helper.GetClientIp()
	if len(ips) > 0 {
		mon.ip = ips[0]
	}
	mon.orderActionRspChan = make(chan MonitorOrderActionRsp, 5)
	var err error
	mon.alerter, err = helper.NewAlert(helper.AlertConfig{
		Channel: helper.ChannelInterface,
	})

	mon.area = tools.MustGetServerArea()
	mi := brokerconfig.LoadMachineInfo()
	if mi != nil {
		mon.Zone = mi.Zone
	}
	if mon.Zone == "" {
		mon.frs.Logger.Errorf("[utf_ign] zone is empty")
	}
	mon.exName = exName
	mon.pair = pair
	mon.exArea = local.GetExchangeLocal(exName)
	mon.crossDomain = mon.exArea == local.GetLocal() || mon.exArea == mon.area

	// res.endPoints = cmap.New[*Endpoint]()
	mon.endPoints = cmap.NewStringer[EndpointKey_T, *Endpoint]()
	mon.entries = cmap.NewStringer[LineAction, *Endpoint]()
	mon.monitorTrigger = monitorTrigger
	mon.stopChan = make(chan struct{}, 2)
	// for _, url := range urls {
	// res.AddEndpoint(url)
	// }

	if strings.HasPrefix(exName, "gate") {
		mon.cidPrefix = "t-" + monitorOrderCidPrefix
	} else {
		mon.cidPrefix = monitorOrderCidPrefix
	}
	mon.cidPrefixLen = len(mon.cidPrefix)
	mon.mockCidForActive = fmt.Sprintf("%s99_%d", mon.cidPrefix, time.Now().UnixNano())

	influxDBConfig := client.HTTPConfig{
		Addr:     "http://" + influxCfg.Addr,
		Username: influxCfg.User,
		Password: influxCfg.Pw,
	}

	mon.influxClient, err = client.NewHTTPClient(influxDBConfig)

	if err != nil {
		log.Warnf("[DelayMonitor] inited influx client failed %s", err.Error())
		push.PushAlert("Init failed", fmt.Sprintf("influx client err %v", err))
		return
	} else {
		log.Infof("[DelayMonitor] inited influx client")
	}

	mon.monitorId = exName + fmt.Sprintf("_%d", time.Now().UnixMicro())
	log.Infof("[DelayMonitor] new inst Created %s", mon.monitorId)
	return
}

// 可以在 InitDelayMonitor调用之前调用
func (m *DelayMonitor) AddLine(line Line) bool {
	if m.lines == nil {
		m.lines = make([]Line, 0)
	}
	for _, l := range m.lines {
		if l.String() == line.String() {
			panic("duplicate add line. " + line.String())
		}
	}
	if helper.DEBUGMODE {
		if m.frs == nil || m.frs.Logger == nil {
			log.Debugf("add line: %v", line)
		} else {
			m.frs.Logger.Debugf("add line: %v", line)
		}
	}
	m.lines = append(m.lines, line)
	return true
}
func (m *DelayMonitor) TryNext(rsp MonitorOrderActionRsp) bool {
	if m.monitorId == "" {
		return false
	}
	if helper.DEBUGMODE {
		m.frs.Logger.Debugf("TryNext. rsp %v", rsp)
	}
	if m.IsMonitorOrder(rsp.Cid) {
		// opt 尽量计算交易所收到请求的时间，不是我们收到回报的时间
		if rsp.Action == ActionType_Place && rsp.Oid == "" {
			m.frs.Logger.ErrorfWithStacktrace("must fill oid in place action rsp")
			return true
		} else if rsp.Action == ActionType_Amend && rsp.AmendSucc != true {
			m.frs.Logger.ErrorfWithStacktrace("failed to amend order, rsp.AmendSucc is false")
			return true
		}
		m.orderActionRspChan <- rsp
		if helper.DEBUGMODE {
			m.frs.Logger.Debugf("TryNext inserted chan. rsp %v", rsp)
		}
		return true
	} else if rsp.Cid == "" {
		m.frs.Logger.ErrorfWithStacktrace("not set rsp.ClientId")
		return true
	}
	return false
}
func (m *DelayMonitor) Trigger() {
	if !m.monitoring.CompareAndSwap(false, true) {
		msg := fmt.Sprintf("should not monitoring. %v", m.frs.BrokerConfig.Name)
		log.Error(msg)
		m.alerter.Error(msg)
		return
	}
	succ := false
	defer func() {
		if !succ {
			msg := fmt.Sprintf("monitor failed %v", m.frs.BrokerConfig.Name)
			m.frs.Logger.Error(msg)
			m.alerter.Error(msg)
			ordersLeft := false
			for tried := 0; tried < 3; tried++ {
				ordersLeft = m.frs.subRsInner.DoCancelOrdersIfPresent(true)
				if ordersLeft {
					time.Sleep(time.Second)
				} else {
					break
				}
			}
			if ordersLeft {
				msg := fmt.Sprintf("failed to cancel orders %v", m.frs.BrokerConfig.Name)
				m.frs.Logger.Error(msg)
				m.alerter.Error(msg)
			}
		}
		m.monitoring.Store(false)
	}()

	if !m.notSupportSwitchLine {
	outfor:
		for _, l := range m.lines {
			tried := 0
			switched := false
			for ; tried < 3; tried++ {
				err, _ := m.frs.SwitchLine(ActionType_Place, SelectedLine{Line: l})
				if err != nil {
					if strings.Contains(err.Error(), "not support") {
						log.Warnf("[%s] not support switch line. %v", m.frs.BrokerConfig.Name, err)
						m.notSupportSwitchLine = true
						break outfor
					}
					m.frs.Logger.Warnf("failed to switch line, try again. err %v", err)
					time.Sleep(time.Second * 5) // bg出现撤单后还认为有挂单而不能切换仓位模式，所以睡眠长一点
					continue
				}
				switched = true
				break
			}
			if !switched {
				m.frs.Logger.Errorf("failed to switch line final")
				push.PushAlert("Failed Switch", fmt.Sprintf("failed to switch line final. action: %s, line:%v", ActionType_Place, l))
				if m.callExitIfSwitchLineFailed {
					m.frs.Logger.Errorf("call exit")
					m.frs.Cb.OnExit("failed to switch line final")
				}
				continue
			}
			if l.Client == ClientType_Rs {
				// price := m.frs.TradeMsg.Ticker.Bp.Load() * 0.96
				// size := helper.FixAmount(fixed.NewF(m.frs.PairInfo.MinOrderValue.Float()*1.1/price), m.frs.PairInfo.StepSize)
				// if size.LessThan(m.frs.PairInfo.MinOrderAmount) {
				// size = m.frs.PairInfo.MinOrderAmount
				// }

				s := helper.Signal{
					Type: helper.SignalTypeNewOrder,
					// Price:     price,
					// Amount:    size,
					OrderSide: helper.OrderSideKD,
					OrderType: helper.OrderTypeLimit,
					Time:      0,
				}
				s.ClientID = m.frs.DelayMonitor.mockCidForActive
				s.OrderID = "1234"
				m.CancelOrder(l, s)
			}
			price := m.frs.TradeMsg.Ticker.Bp.Load() * 0.96
			usdValueThreshold := m.frs.PairInfo.MinOrderValue.Float() * 1.1
			var size fixed.Fixed
			for tried := 0; ; tried++ {
				// 每次增加50U，确保大于 min order value
				size = helper.FixAmount(fixed.NewF((usdValueThreshold+float64(50*tried))/price), m.frs.PairInfo.StepSize)
				if size.LessThan(m.frs.PairInfo.MinOrderAmount) {
					size = m.frs.PairInfo.MinOrderAmount
				}
				if size.Float()*price > usdValueThreshold {
					break
				}
			}
			s := helper.Signal{
				Type:      helper.SignalTypeNewOrder,
				Price:     price,
				Amount:    size,
				OrderSide: helper.OrderSideKD,
				OrderType: helper.OrderTypeLimit,
				Time:      0,
			}
			m.PlaceOrder(l, &s)
			// ws place有些所会发两次rsp，
			if err := m.WaitWant(l, s.ClientID, ActionType_Place, ""); err != nil {
				log.Errorf("%v", err)
				return
			}
			s.OrderID = m.curOid // 在place WaitWant中会填入
			// amend里面用WaitWant
			if err := m.AmendOrder(l, s); err != nil {
				log.Errorf("%v", err)
				return
			}
			if err := m.CancelOrder(l, s); err != nil {
				log.Errorf("%v", err)
				return
			}
			time.Sleep(time.Second * 2) // 避免太快切换marginmode失败
		}
	}
	succ = true
}
func (m *DelayMonitor) PlaceOrder(l Line, s *helper.Signal) {
	switch l.Client {
	case ClientType_Rs:
		switch l.Link {
		case LinkType_Colo:
			s.ClientID = m.frs.DelayMonitor.GenCid(EndpointKey_RsColoPlace, false)
			m.frs.subRsInner.DoPlaceOrderRsColo(m.frs.PairInfo, *s)
		case LinkType_Nor:
			s.ClientID = m.frs.DelayMonitor.GenCid(EndpointKey_RsNorPlace, false)
			m.frs.subRsInner.DoPlaceOrderRsNor(m.frs.PairInfo, *s)
		}
	case ClientType_Ws:
		switch l.Link {
		case LinkType_Colo:
			s.ClientID = m.frs.DelayMonitor.GenCid(EndpointKey_WsColoPlace, false)
			m.frs.subRsInner.DoPlaceOrderWsColo(m.frs.PairInfo, *s)
		case LinkType_Nor:
			s.ClientID = m.frs.DelayMonitor.GenCid(EndpointKey_WsNorPlace, false)
			m.frs.subRsInner.DoPlaceOrderWsNor(m.frs.PairInfo, *s)
		}
	}
}
func (m *DelayMonitor) doAmendOrder(l Line, s helper.Signal) (amendSent bool) {
	if l.Client == ClientType_Rs {
		if l.Link == LinkType_Colo && m.frs.Features.DoAmendOrderRsColo {
			m.frs.subRsInner.DoAmendOrderRsColo(m.frs.PairInfo, s)
			amendSent = true
		} else if l.Link == LinkType_Nor && m.frs.Features.DoAmendOrderRsNor {
			m.frs.subRsInner.DoAmendOrderRsNor(m.frs.PairInfo, s)
			amendSent = true
		}
	} else if l.Client == ClientType_Ws {
		if l.Link == LinkType_Colo && m.frs.Features.DoAmendOrderWsColo {
			m.frs.subRsInner.DoAmendOrderWsColo(m.frs.PairInfo, s)
			amendSent = true
		} else if l.Link == LinkType_Nor && m.frs.Features.DoAmendOrderWsNor {
			m.frs.subRsInner.DoAmendOrderWsNor(m.frs.PairInfo, s)
			amendSent = true
		}
	}
	return
}
func (m *DelayMonitor) amendOrderPriceDown(l Line, oriSig, s helper.Signal) (amendSent bool) {
	if s.Price == 0 {
		s.Price = m.frs.TradeMsg.Ticker.Bp.Load() * 0.92 // 比下单时价格距离更低
	} else {
		s.Price -= 2 * m.frs.PairInfo.TickSize
	}
	return m.doAmendOrder(l, s)
}
func (m *DelayMonitor) amendOrderPriceUpAmountUp(l Line, oriSig, s helper.Signal) (amendSent bool) {
	if s.Price == 0 {
		s.Price = m.frs.TradeMsg.Ticker.Bp.Load() * 0.94 // 比下单时价格距离更低
	} else {
		s.Price += 2 * m.frs.PairInfo.TickSize
	}
	s.Amount = s.Amount.Add(fixed.NewI(2, 0).Mul(fixed.NewF(m.frs.PairInfo.StepSize)))
	return m.doAmendOrder(l, s)
}
func (m *DelayMonitor) AmendOrder(l Line, s helper.Signal) error {
	oriSig := s
	if m.amendOrderPriceDown(l, oriSig, s) {
		if err := m.WaitWant(l, s.ClientID, ActionType_Amend, "_pd"); err != nil {
			return err
		}
	}
	// 这些所不能频繁改单，要等一下
	if slice.Contain[string]([]string{helper.BrokernameBybitUsdtSwap.String()}, m.frs.BrokerConfig.Name) {
		time.Sleep(time.Second)
	}
	if m.amendOrderPriceUpAmountUp(l, oriSig, s) {
		if err := m.WaitWant(l, s.ClientID, ActionType_Amend, "_puau"); err != nil {
			return err
		}
	}
	return nil
}
func (m *DelayMonitor) CancelOrder(l Line, s helper.Signal) error {
	switch l.Client {
	case ClientType_Rs:
		switch l.Link {
		case LinkType_Colo:
			// 可以分拆by cid / oid
			m.frs.subRsInner.DoCancelOrderRsColo(m.frs.PairInfo, s)
		case LinkType_Nor:
			m.frs.subRsInner.DoCancelOrderRsNor(m.frs.PairInfo, s)
		}
	case ClientType_Ws:
		switch l.Link {
		case LinkType_Colo:
			m.frs.subRsInner.DoCancelOrderWsColo(m.frs.PairInfo, s)
		case LinkType_Nor:
			m.frs.subRsInner.DoCancelOrderWsNor(m.frs.PairInfo, s)
		}
	}
	return m.WaitWant(l, s.ClientID, ActionType_Cancel, "")
}

// todo 可以分拆by cid / oid
// 多次 撤单
func (m *DelayMonitor) CancelOrderMulti(l Line, s helper.Signal) error {
	if m.cancelOrder(l, s) {
		// if err := m.WaitWant(l, s.ClientID, ActionType_Cancel, "_oid"); err != nil {
		return m.WaitWant(l, s.ClientID, ActionType_Cancel, "")
	} else {
		return errors.New("not sent cancel order req")
	}
	// todo 可以分拆by cid / oid
}
func (m *DelayMonitor) cancelOrder(l Line, s helper.Signal) (sent bool) {
	switch l.Client {
	case ClientType_Rs:
		switch l.Link {
		case LinkType_Colo:
			// 可以分拆by cid / oid
			m.frs.subRsInner.DoCancelOrderRsColo(m.frs.PairInfo, s)
			sent = true
		case LinkType_Nor:
			m.frs.subRsInner.DoCancelOrderRsNor(m.frs.PairInfo, s)
			sent = true
		}
	case ClientType_Ws:
		switch l.Link {
		case LinkType_Colo:
			m.frs.subRsInner.DoCancelOrderWsColo(m.frs.PairInfo, s)
			sent = true
		case LinkType_Nor:
			m.frs.subRsInner.DoCancelOrderWsNor(m.frs.PairInfo, s)
			sent = true
		}
	}
	return
}

func (m *DelayMonitor) WaitWant(l Line, cid string, action ActionType, actionParamExt string) error {
	if helper.DEBUGMODE {
		m.frs.Logger.Debugf("waiting. line %v, cid %v, action %v, actionExt %v", l, cid, action, actionParamExt)
	}
	// 下单会有多阶段提交，多次rsp，所以用for
	for {
		select {
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout when wait. line %v, action %v, cid %v", l, action, cid)
		case rsp := <-m.orderActionRspChan:
			if helper.DEBUGMODE {
				m.frs.Logger.Debugf("received rsp %v", rsp)
			}
			if rsp.Action == ActionType_Cancel && rsp.Cid == m.mockCidForActive {
				if helper.DEBUGMODE {
					m.frs.Logger.Debugf("it is cancel active. rsp %v", rsp)
				}
				return nil
			}
			if rsp.Client != l.Client || rsp.Cid != cid || rsp.Action != action {
				m.frs.Logger.Errorf("wrong rsp when wait, want (line %v, action %v, cid %v), actual is %v", l, action, cid, rsp)
				continue
			}
			if action == ActionType_Amend && rsp.AmendSucc != true {
				m.frs.Logger.Errorf("failed to amend order, rsp.AmendSucc is false")
				continue
			}
			if action == ActionType_Place {
				if rsp.Oid == "" {
					m.frs.Logger.Errorf("must fill oid in place rsp")
					continue
				}
				m.curOid = rsp.Oid // 填充 oid
			}
			if helper.DEBUGMODE {
				m.frs.Logger.Debugf("matched WaitWant rsp (line %v, action %v, cid %v), rsp is %v", l, action, cid, rsp)
			}
			m.updateLineActionDelayWithDuration(l, action.String()+actionParamExt, rsp.DurationUs)
			return nil
		}
	}
}

func NewSelectLineFromStr(str string) (s SelectedLine, err error) {
	vals := strings.Split(str, ";")

	for _, v := range vals {
		vals2 := strings.Split(v, ":")
		switch strings.ToLower(strings.TrimSpace(vals2[0])) {
		case "client":
			s.Client = ClientType(strings.TrimSpace(vals2[1]))
		case "link":
			s.Link = LinkType(strings.TrimSpace(vals2[1]))
			// case "mm":
			// !!! 保证金模式只提供建议，不可在底层切换
		}
	}
	if s.Client == ClientType_Nil {
		err = errors.New("miss client type")
	}
	if s.Link == LinkType_Nil {
		err = errors.New("miss link type")
	}
	return
}

type ActionType string

func (a ActionType) String() string {
	return string(a)
}

const (
	ActionType_Place  ActionType = "place"
	ActionType_Cancel ActionType = "cancel"
	ActionType_Amend  ActionType = "amend"
)

type EndpointConfiguration struct {
	LinkType             LinkType
	ClientType           ClientType
	MarginModeDeprecated helper.MarginMode // c: cross, i:isolate
	Action               ActionType
}
type Endpoint struct {
	// EndpointConfiguration
	LineAction
	monitor *DelayMonitor
	// url      string
	isActive bool
	// All in microseconds
	lastUpdateOrder int64
	delay           PassTimeRingBuffer
	tags            map[string]string
}

func (e *Endpoint) Tags() map[string]string {
	if e.tags != nil {
		return e.tags
	}
	e.tags = e.monitor.Tags()
	e.tags["link"] = string(e.Line.Link)
	e.tags["client"] = string(e.Line.Client)
	e.tags["action"] = string(e.ActionWithExt)
	e.tags["margin_mode"] = e.Line.MarginMode.String()
	// e.tags["link"] = string(e.LinkType)
	// e.tags["client"] = string(e.ClientType)
	// e.tags["action"] = string(e.Action)
	// switch e.MarginModeDeprecated {
	// case helper.MarginMode_Cross:
	// e.tags["margin_mode"] = "c"
	// case helper.MarginMode_Iso:
	// e.tags["margin_mode"] = "i"
	// }
	return e.tags
}
func (mon *DelayMonitor) AddDefaultEndpoints(auto bool, epc EndpointConfiguration) {
	mon.AddEndpoint(EndpointKey_RsNorPlace, true, epc)
	mon.AddEndpoint(EndpointKey_RsNorCancel, true, epc)
	mon.AddEndpoint(EndpointKey_WsNorPlace, true, epc)
	mon.AddEndpoint(EndpointKey_WsNorCancel, true, epc)
	mon.AddEndpoint(EndpointKey_RsColoPlace, true, epc)
	mon.AddEndpoint(EndpointKey_RsColoCancel, true, epc)
	mon.AddEndpoint(EndpointKey_WsColoPlace, true, epc)
	mon.AddEndpoint(EndpointKey_WsColoCancel, true, epc)

	mon.AddEndpoint(EndpointKey_RsNorAmend, true, epc)
	mon.AddEndpoint(EndpointKey_WsNorAmend, true, epc)
	mon.AddEndpoint(EndpointKey_RsColoAmend, true, epc)
	mon.AddEndpoint(EndpointKey_WsColoAmend, true, epc)
}

// auto 根据key类型自动设置epc
func (mon *DelayMonitor) AddEndpoint(key EndpointKey_T, auto bool, epc EndpointConfiguration) {
	return
	// todo remove
	if mon.monitorId == "" {
		log.Warn("[DelayMonitor] not inited")
		return
	}
	if auto {
		epc.ClientType, epc.LinkType, epc.Action = key.GetFactors()
	}

	// mon.endPoints.Set(key, &Endpoint{
	// EndpointConfiguration: epc,
	// url:                   key,
	// monitor: mon,
	// delay:   NewPassTimeRingBuffer(),
	// })
	// log.Debugf("[DelayMonitor] url added %s", key)
}

func (mon *DelayMonitor) Run() {
	if mon.monitorId == "" {
		log.Warn("[DelayMonitor] not inited")
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second * time.Duration(QUERY_DELAY_SEC))
		time.Sleep(10 * time.Second)
		ticker2 := time.NewTicker(time.Second * time.Duration(SEND_DELAY_SEC))
		defer ticker.Stop()
		defer ticker2.Stop()
		for {
			select {
			case <-ticker.C:
				mon.Trigger()
				continue // 使用状态机
				for i := range mon.endPoints.IterBuffered() {
					if i.Val.isActive {
						if time.Now().UnixMicro()-i.Val.lastUpdateOrder > RESET_DELAY_SEC*1e6 && i.Val.lastUpdateOrder > 0 {
							i.Val.delay.Reset()
							log.Warnf("[DelayMonitor][%s]:lastUpdateOrder[%d] too old, reset ", mon.monitorId, i.Val.lastUpdateOrder)
						}
					} else {
						if helper.DEBUGMODE {
							log.Debugf("[DelayMonitor][%s]: call monitorTrigger(%v)", mon.monitorId, i.Key)
						}
						mon.monitorTrigger(i.Key, true)
						time.Sleep(2 * time.Second)
					}
				}
			case <-mon.stopChan:
				log.Infof("[DelayMonitor][%s]:exiting, DelayStats us: %v", mon.monitorId, mon.String())
				return
			case <-ticker2.C:
				if helper.DEBUGMODE {
					log.Infof("[DelayMonitor][%s]:sending to influx %v", mon.monitorId, mon.String())
				}
				if !IsUtf {
					mon.writeToInfluxDB()
				}
			}
		}
	}()
}

func (mon *DelayMonitor) writeToInfluxDB() {
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database: "delay_monitor",
	})
	if err != nil {
		log.Errorf("[DelayMonitor] init NewBatchPoints failed %s", err.Error())
	}

	got := false
	for item := range mon.entries.IterBuffered() {
		// for item := range mon.endPoints.IterBuffered() {
		if item.Val.delay.Count == 0 {
			continue
		}
		// item.Val.delay.
		orderMin, orderMax, orderCount, orderLast, orderAvg := item.Val.delay.GetMinMaxCountLastAvg()
		// activated := "false"
		// if item.Val.isActive {
		// activated = "true"
		// }
		tags := item.Val.Tags()
		p, _ := client.NewPoint("delay",
			tags, map[string]interface{}{
				"avg":   orderAvg,
				"min":   orderMin,
				"max":   orderMax,
				"last":  orderLast,
				"count": orderCount,
			})
		log.Debugf("[DelayMonitor] influx add point  %v", p.String())
		bp.AddPoint(p)
		item.Val.delay.Reset()
		got = true
	}

	if !got {
		return
	}

	err = mon.influxClient.Write(bp)
	if err != nil {
		log.Errorf("[DelayMonitor] write influx failed %s", err)
		push.PushAlert("Write influx failed", fmt.Sprintf("err: %v", err))
	} else {
		if helper.DEBUGMODE {
			log.Debugf("[DelayMonitor] write influx succ %s,  %v", mon.exName, bp)
		}
	}
}
func (mon *DelayMonitor) IsActiveReq(cid string) bool {
	if mon.monitorId == "" {
		return false
	}
	return cid == mon.mockCidForActive
}

func (mon *DelayMonitor) IsMonitorOrder(cid string) bool {
	if mon.monitorId == "" {
		return false
	}
	if !strings.HasPrefix(cid, mon.cidPrefix) {
		return false
	}
	return true
}
func (mon *DelayMonitor) GenCid(url EndpointKey_T, activeFirst bool) string {
	if mon.monitorId == "" {
		return ""
	}
	// like t-M_02_1722938418
	return fmt.Sprintf("%s%.2d_%d", mon.cidPrefix, int(url), time.Now().UnixMicro())
	// ========== 使用 active oder =========
	// activeStr := "a"
	// if !activeFirst {
	// 	activeStr = "n"
	// }
	// like t-M_a_02_1722938418
	// return fmt.Sprintf("%s%s_%.2d_%d", mon.cidPrefix, activeStr, int(url), time.Now().UnixMicro())
	// ======================
}
func (mon *DelayMonitor) GetActiveAndKeyFromCid(cid string) (isActiveOrder bool, key EndpointKey_T, isMonitorOrder bool) {
	if !mon.IsMonitorOrder(cid) {
		return
	}
	// ========== 使用 active oder =========
	// if cid[mon.cidPrefixLen:mon.cidPrefixLen+1] == "a" {
	// 	isActiveOrder = true
	// }
	// s := mon.cidPrefixLen + 2
	// ============= end ==============
	s := mon.cidPrefixLen
	str := cid[s : s+2]
	// i := strings.Index(str, "_")
	k, err := strconv.Atoi(str)
	if err != nil {
		log.Errorf("failed to conver. %v, %v", cid, err)
		return
	}
	key = EndpointKey_T(k)
	if helper.DEBUGMODE {
		log.Debugf("key for cid %s is : %v", cid, key)
	}
	isMonitorOrder = true
	return
}

func (mon *DelayMonitor) UpdateDelay(url EndpointKey_T, startUs int64) {
	mon.update(url, time.Now().UnixMicro()-startUs)
}
func (mon *DelayMonitor) updateLineActionDelayWithDuration(line Line, actionWithExt string, durationUs int64) {
	// if mon.monitorId == "" {
	// return
	// }
	la := LineAction{Line: line, ActionWithExt: actionWithExt}
	endpoint, ok := mon.entries.Get(la)
	if !ok {
		endpoint = &Endpoint{LineAction: la, monitor: mon}
		endpoint.delay = NewPassTimeRingBuffer()
		mon.entries.Set(la, endpoint)
	}
	endpoint.delay.Put(durationUs)
	endpoint.lastUpdateOrder = time.Now().UnixMicro()
	if helper.DEBUGMODE {
		mon.frs.Logger.Debugf("update delay. endpoint %v, %v ", la, endpoint.delay.String())
	}
}
func (mon *DelayMonitor) UpdateDelayWithDuration(url EndpointKey_T, durationUs int64) {
	mon.update(url, durationUs)
}

// todo
func (mon *DelayMonitor) UpdateDelayStr(url EndpointKey_T, cidWithStartUs string) {
	mon.update(url, time.Now().UnixMicro()-mon.parseStr2Delay(cidWithStartUs))
}

func (mon *DelayMonitor) update(key EndpointKey_T, delay int64) {
	if mon.monitorId == "" {
		return
	}
	endpoint, ok := mon.endPoints.Get(key)
	if ok {
		endpoint.delay.Put(delay)
		endpoint.lastUpdateOrder = time.Now().UnixMicro()
	} else {
		log.Warnf("[DelayMonitor][%s]: delay update not found key [%s]", mon.monitorId, key)
	}
}

//	func (mon *DelayMonitor) Activate(url string, deactivateOthers bool) {
//		if mon.monitorId == "" {
//			log.Warn("[DelayMonitor] not inited")
//			return
//		}
//		endpoint, ok := mon.endPoints.Get(url)
//		if ok {
//			if endpoint.isActive {
//				log.Warnf("[DelayMonitor][%s]: URL to active was alreay activated [%s]", mon.monitorId, url)
//			} else {
//				endpoint.isActive = true
//			}
//		} else {
//			log.Warnf("[DelayMonitor][%s]: Not found URL to active [%s]", mon.monitorId, url)
//		}
//		if deactivateOthers {
//			for i := range mon.endPoints.IterBuffered() {
//				if i.Val.url != url {
//					i.Val.isActive = false
//				}
//			}
//		}
//	}
//
//	func (mon *DelayMonitor) Deactivate(url string) {
//		if mon.monitorId == "" {
//			log.Warn("[DelayMonitor] not inited")
//			return
//		}
//		endpoint, ok := mon.endPoints.Get(url)
//		if ok {
//			if !endpoint.isActive {
//				log.Warnf("[DelayMonitor][%s]:URL to deactive was alreay deactivated [%s]", mon.monitorId, url)
//			} else {
//				endpoint.isActive = false
//			}
//		} else {
//			log.Warnf("[DelayMonitor][%s]:Not found URL to deactive [%s]", mon.monitorId, url)
//		}
//	}
func (mon *DelayMonitor) Stop() {
	if mon.stopChan != nil {
		mon.stopChan <- struct{}{}
	}
	if mon.influxClient != nil {
		mon.influxClient.Close()
	}
}

// func (mon *DelayMonitor) GetOrderMonitorDelay(reset bool) (orderDelay map[string]int64, cancelDelay map[string]int64) {
// 	if mon.monitorId == "" {
// 		log.Warn("[DelayMonitor] not inited")
// 		return
// 	}
// 	orderDelay = make(map[string]int64)
// 	cancelDelay = make(map[string]int64)
// 	for i := range mon.endPoints.IterBuffered() {
// 		_, _, _, _, orderDelay[i.Key] = i.Val.delayOrder.GetMinMaxCountLastAvg()
// 		_, _, _, _, cancelDelay[i.Key] = i.Val.delayCancel.GetMinMaxCountLastAvg()
// 		if reset {
// 			i.Val.delayOrder.Reset()
// 			i.Val.delayCancel.Reset()
// 		}
// 	}
// 	return
// }

func (mon *DelayMonitor) String() string {
	if mon.monitorId == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(mon.monitorId)
	sb.WriteString("\n")
	// for i := range mon.endPoints.IterBuffered() {
	for i := range mon.entries.IterBuffered() {
		sb.WriteString("Endpoint: ")
		sb.WriteString(i.Key.String())
		sb.WriteString(", ")
		// if i.Val.isActive {
		// 	sb.WriteString(", IsActive:true, ")
		// } else {
		// 	sb.WriteString(", IsActive:false, ")
		// }
		sb.WriteString("delay:")
		// sb.WriteString(fmt.Sprintf("%d", i.Val.delay.GetDelay()))
		// sb.WriteString(", CancelDelayAvg:")
		// sb.WriteString("\n")
		sb.WriteString(i.Val.delay.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// const US_TS_LEN = 10 + 6 // 微秒时间戳长度
func (mon *DelayMonitor) parseStr2Delay(cidWithStartUs string) int64 {
	if mon.IsMonitorOrder(cidWithStartUs) {
		// M_a_02_1723090454326104
		start, err := strconv.ParseInt(cidWithStartUs[mon.cidPrefixLen+5:], 10, 64)
		if err != nil {
			log.Errorf("[DelayMonitor][%s]:Update order string not a delay string %v", mon.monitorId, cidWithStartUs)
		}
		return start
	} else {
		log.Errorf("[DelayMonitor][%s]:Update order string not a delay string %v", mon.monitorId, cidWithStartUs)
	}
	return 0
}

//没必要，满了pushkick 一样要iterate 来find minmax
/*------------------------------------------------------------------------------------------------------------------*/
// PassTimeRingBuffer 统计延迟情况
type PassTimeRingBuffer struct {
	sync.RWMutex
	Pass  *cqueue.CircularQueue[int64] //pass.len not reliable as reset does not reset pass
	Count int
}

func NewPassTimeRingBuffer() PassTimeRingBuffer {
	return PassTimeRingBuffer{Pass: cqueue.NewCircularQueue[int64](RB_SIZE)}
}

func (p *PassTimeRingBuffer) Reset() {
	p.Lock()
	p.Pass = cqueue.NewCircularQueue[int64](RB_SIZE)
	p.Count = 0
	p.Unlock()
}

func (p *PassTimeRingBuffer) Put(in int64) {
	p.Lock()
	p.Pass.PushKick(in)
	if p.Count < RB_SIZE {
		p.Count++
	}
	p.Unlock()
}

func (p *PassTimeRingBuffer) String() string {
	min, max, count, last, avg := p.GetMinMaxCountLastAvg()
	return fmt.Sprintf("(us): max:%d, min:%d, ave:%d, count:%d last:%d", max, min, avg, count, last)
}

func (p *PassTimeRingBuffer) GetDelay() int64 {
	_, _, _, _, avg := p.GetMinMaxCountLastAvg()
	return avg
}

func (p *PassTimeRingBuffer) GetMinMaxCountLastAvg() (min, max int64, count int64, last int64, avg int64) {
	p.RLock()
	defer p.RUnlock()
	if p.Pass.Len() == 0 || p.Pass.IsEmpty() {
		return
	}
	count = int64(p.Pass.Len())

	min = math.MaxInt64
	max = 0
	last = 0

	var sum int64
	for i := 0; i < p.Pass.Len(); i++ {
		num := p.Pass.GetElement(i)
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
		last = num
		sum += num
	}

	avg = (sum) / (count)
	return
}
