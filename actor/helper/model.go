// 适合所有策略的通用数据结构放在这里
package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"actor/broker/base/orderbook_mediator"
	"actor/third/cmap"
	"actor/third/fixed"
	jsoniter "github.com/json-iterator/go"

	"strings"
	"sync"

	"actor/third/log"
	"go.uber.org/atomic"
)

// 注意！应用层不要传入，client接口会自动创建
type PriData struct {
	Position    *Pos
	PositionMap cmap.ConcurrentMap[string, *Pos]         // 只可从 symbol 查找，不可从 pair 查找.
	EquityMapRs cmap.ConcurrentMap[string, *EquityEvent] // key: asset name, 小写. rs/ws使用不同EquityMap
	EquityMapWs cmap.ConcurrentMap[string, *EquityEvent] // key: asset name, 小写. rs/ws使用不同EquityMap
}
type BrokerConfig struct {
	Name        string // 一定填盘口名称
	AccountName string // 账号名字
	AccessKey   string
	SecretKey   string
	PassKey     string
	// broker code  通过 http headers/body 传递给交易所的 broker id 走这个方案， 走 cid broker 方案请看 cidTool
	// 这个字段来自前端下发参数 bb bg 走 headers apiBrokerCode 方案  hb_swap 走 body apiBrokerCode 方案 bin htx_Spot 走 citTool 方案
	ApiBrokerCode string // 一定要仔细阅读本段注释
	ProxyURL      string // 代理地址 example: socks5://127.0.0.1:1080 | http://127.0.0.1:1080
	LocalAddr     string // 目标发包本地内网ip地址
	// 订阅频道选择
	NeedTicker            bool // 是否需要bbo
	NeedIndex             bool // 是否需要index price & mark price
	PushTickerEvenEmpty   bool // bbo为空时仍然推送
	NeedTrade             bool // 是否需要共有成交
	NeedAuth              bool // 是否需要私有连接 包括账户资金 仓位 订单
	NeedMarketLiquidation bool // 需要市场爆仓单
	// 订单簿的实现准则顺序：1. 尽力满足档位数  2. 快（全量快就用全量）
	WsDepthLevel           int    // 需要订阅的depth 档位数，0表示不用订阅。越小档位数越快
	DisableAutoUpdateDepth bool   // 是否禁用自动更新depth，默认不禁用
	BanColo                bool   // 是否禁用colo，默认不禁用
	BanRsWs                bool   // 是否禁用 rs里面的 req ws
	AlertCarryInfo         string // 出现错误底层要发送报警时，附带这条信息。一般是taskUid\ip之类，上层组装
	ExcludeIpsAsPossible   []string
	MaxValueClosePerTimes  float64    // 清仓时每次下单的最大 U 价值
	Logger                 log.Logger `json:"-"` // 必须填
	RootLogger             log.Logger `json:"-"` // 必须填, 部分log会打印到这里
	IsBorrowFund           bool       // 是否借贷账户
	RobotId                string     // 机器人ID
	DisableLineSwitcher    bool       // 关闭line switcher

	// 应对event化添加
	Pairs     []Pair // 必须一个以上。高频模式：只有1个且SymbolAll为false
	SymbolAll bool   // true表示使用全币对模式，取Pairs[0]用于初始化
	//
	ActivateDelayMonitor bool // 全链路监控打开

	OwnerDeerKeys                 []string
	CallExitIfSwitchLineFailed    bool       // 切换line失败时，是否调用exit
	IgnoreDuplicateClientOidError bool       // 忽略重复的clientOid
	IsBgSpec                      bool       // 多链路下单
	InitialAcctConfig             AcctConfig // 账户配置
	RawDataMmapCollectPath        string     // 原始数据 mmap 收集路径
	Need1MinKline                 bool
}

// 不可高频调用
func (bc *BrokerConfig) String() string {
	// return fmt.Sprintf("{Name:%s,Pair:%s,NeedTicker:%v,NeedTickerEvenEmpty:%v,WsDepthLevel:%v, NeedDepth:%v,NeedPartial:%v,NeedTrade:%v,NeedAuth:%v,BanColo:%v,AlertCarryInfo:%v,MaxValueClosePerTimes:%v}",
	// bc.Name, bc.Pair, bc.NeedTicker, bc.PushTickerEvenEmpty, bc.WsDepthLevel, bc.WsDepthLevel > 0, bc.NeedPartialDeprecated, bc.NeedTrade, bc.NeedAuth, bc.BanColo, bc.AlertCarryInfo, bc.MaxValueClosePerTimes)
	return bc.StringLite()
}

// 不可高频调用
func (bc *BrokerConfig) StringLite() string {
	var bcNew BrokerConfig = *bc
	bcNew.SecretKey = "ERASE"
	bcNew.PassKey = "ERASE"
	b, err := json.Marshal(bcNew)
	if err != nil {
		log.Error("failed to marshal bc. %v", err)
		return ""
	}
	return string(b)
}

func (bc *BrokerConfig) NeedPubWs() bool {
	return bc.NeedIndex || bc.NeedTicker || bc.NeedTrade || bc.WsDepthLevel > 0 || bc.NeedMarketLiquidation || bc.Need1MinKline
}

// broker的配置文件
type BrokerConfigExt struct {
	BrokerConfig
	PriData
}

// 不可高频调用
func (bc *BrokerConfigExt) String() string {
	// return fmt.Sprintf("{Name:%s,Pair:%s,NeedTicker:%v,NeedTickerEvenEmpty:%v,WsDepthLevel:%v, NeedDepth:%v,NeedPartial:%v,NeedTrade:%v,NeedAuth:%v,BanColo:%v,AlertCarryInfo:%v,MaxValueClosePerTimes:%v}",
	// bc.Name, bc.Pair, bc.NeedTicker, bc.PushTickerEvenEmpty, bc.WsDepthLevel, bc.WsDepthLevel > 0, bc.NeedPartialDeprecated, bc.NeedTrade, bc.NeedAuth, bc.BanColo, bc.AlertCarryInfo, bc.MaxValueClosePerTimes)
	return bc.StringLite()
}

// 不可高频调用
func (bc *BrokerConfigExt) StringLite() string {
	var bcNew BrokerConfigExt = *bc
	bcNew.SecretKey = "ERASE"
	bcNew.PassKey = "ERASE"
	b, err := json.Marshal(bcNew)
	if err != nil {
		log.Error("failed to marshal bc. %v", err)
		return ""
	}
	return string(b)
}

// easyjson:json
type Seq_T struct {
	Ex            atomic.Int64 `json:"e"`  // 交易所返回序列号，能明确表示递增的字段，有可能是时间戳，注意不同接口返回的时间戳是不是同一个
	Inner         atomic.Int64 `json:"i"`  // 我们系统序列号，强制使用tsns timestamp
	InnerServerId int64        `json:"ii"` // 内部服务器id，用ip转int
	Cnt           atomic.Int64 `json:"c"`  // 计数器
}

func NewSeq(ex, inner int64) (seq Seq_T) {
	seq.Ex.Store(ex)
	seq.Inner.Store(inner)
	seq.InnerServerId = InnerServerId
	return
}

// 换用 easyjson
// func (u Seq) MarshalJSON() ([]byte, error) {
// 	return json.Marshal(
// 		map[string]interface{}{
// 			"e":  u.Ex.Load(),
// 			"i":  u.Inner.Load(),
// 			"ii": u.InnerServerId,
// 		})
// }
// func (u *Seq) UnmarshalJSON(data []byte) error {
// 	seq := &struct {
// 		Ex            int64 `json:"e"`
// 		Inner         int64 `json:"i"`
// 		InnerServerId int64 `json:"ii"`
// 	}{}
// 	if err := json.Unmarshal(data, &seq); err != nil {
// 		return err
// 	}
// 	u.Ex.Store(seq.Ex)
// 	u.Inner.Store(seq.Inner)
// 	u.InnerServerId = seq.InnerServerId
// 	return nil
// }

func (s *Seq_T) String() string {
	return fmt.Sprintf("Seq{Ex:%d Inner:%d InnerServerId:%d}", s.Ex.Load(), s.Inner.Load(), s.InnerServerId)
}
func (s *Seq_T) CompositeInt64() int64 {
	return s.Ex.Load()*100 + s.Inner.Load()*10 + s.InnerServerId
}

// 传入的seq是否较大，同时更新新值
func (s *Seq_T) NewerAndStore(seqEx, seqInner int64, serverId ...int64) bool {
	// 放弃检查，因为dark用了tsc而非ns
	// _ = DEBUGMODE && (seqInner == 0 || MustNanos(seqInner))
	oe := s.Ex.Load()
	if seqEx > oe { // 大概率
		s.Ex.Store(seqEx)
		s.Inner.Store(seqInner)
		s.Cnt.Add(1)
		return true
	} else if seqEx == oe && (len(serverId) == 0 || serverId[0] == 0 || serverId[0] == s.InnerServerId) && seqInner > s.Inner.Load() { // 避免跨服务器对比
		s.Ex.Store(seqEx)
		s.Inner.Store(seqInner)
		s.Cnt.Add(1)
		return true
	}
	return false
}
func (s *Seq_T) IncreaseExSeq(increment int64) bool {
	s.Ex.Add(increment)
	return false
}
func (s *Seq_T) IncreaseInnerSeq(increment int64) bool {
	s.Inner.Add(increment)
	return false
}
func (s *Seq_T) NewerAndStoreEntity(s1 *Seq_T) bool {
	if s.NewerAndStore(s1.Ex.Load(), s1.Inner.Load(), s1.InnerServerId) {
		s.InnerServerId = s1.InnerServerId
		return true
	}
	return false
}

// 负数表示rebate我们收入
type Fee struct {
	Maker float64 // like -0.00015
	Taker float64 // taker费率，like 0.0002
}

// 列表使用的订单结构
type OrderForList struct {
	Symbol        string
	ClientID      string      // 客户定义的id
	OrderID       string      // 交易所的id
	Price         float64     // 下单价格
	Amount        fixed.Fixed // 数量
	OrderSide     OrderSide   // 订单方向
	OrderType     OrderType   // 订单类型
	OrderState    OrderState  // 订单当前状态
	CreatedTimeMs int64       // 单位毫秒
	UpdatedTimeMs int64       // 订单更新时间，毫秒
	Filled        fixed.Fixed
	FilledPrice   float64
	Text          string // 内部订单备注
}

type DealForList struct {
	Symbol          string
	Pair            string
	ClientID        string      // 客户定义的id
	OrderID         string      // 交易所的id
	DealID          string      // 交易id
	OrderSide       OrderSide   // 订单方向
	TradeTimeMs     int64       // 交易发生时间
	FilledThis      fixed.Fixed //本次成交数量
	FilledPriceThis float64     //本次成交均价
	Fee             float64
	IsTaker         bool
	CommissionAsset string // 佣金资产
}

type DealListResponse struct {
	Deals   []DealForList // 根据订单创建时间，从旧到新排序
	HasMore bool          // 表示在给定的时间范围内，是否还有订单没返回。如果是true，调用者可以根据Data最后一个订单的时间发起新请求
}

// 低性能，少用
func (o OrderForList) StringHeavy() string {
	m, _ := jsoniter.Marshal(o)
	return fmt.Sprintf("OrderForList(%s)", string(m))
}

type OrderListResponse struct {
	Orders  []OrderForList // 根据订单创建时间，从旧到新排序
	HasMore bool           // 表示在给定的时间范围内，是否还有订单没返回。如果是true，调用者可以根据Data最后一个订单的时间发起新请求
}

// pair 本地交易对数据结构
// 注意！不可使用 == 对比，只能用Equal()
// easyjson:json
type Pair struct {
	Base         string        `json:"b,omitempty"` // 交易标的 默认小写
	Quote        string        `json:"q,omitempty"` // 计价货币 默认小写
	More         string        `json:"m,omitempty"` // 更多信息 用于交割合约等特殊情况
	Output       string        `json:"o,omitempty"` // 直接得到小写字符串
	ExchangeInfo *ExchangeInfo `json:"-"`
}

func (p Pair) ToString() string {
	return p.String()
}
func (p *Pair) String() string {
	if p.Output != "" {
		return p.Output
	}
	if p.More == "" {
		p.Output = fmt.Sprintf("%s_%s", p.Base, p.Quote)
	} else {
		p.Output = fmt.Sprintf("%s_%s_%s", p.Base, p.Quote, p.More)
	}
	return p.Output
}

func (p *Pair) Equal(_p Pair) bool {
	return p.Base == _p.Base && p.Quote == _p.Quote && p.More == _p.More
}

func NewPair(base, quote, more string) Pair {
	if base == "" || quote == "" {
		log.Errorf("[Pair] 格式错误 base:%v, quote:%v, more:%v", base, quote, more)
		return Pair{}
	}
	// 正则表达式判断 base 是否数字和字母
	matched, err := regexp.MatchString("^[0-9a-zA-Z]+$", base)
	if err != nil {
		log.Errorf("[Pair] base 格式错误 base:%v, quote:%v, more:%v", base, quote, more)
		return Pair{}
	}
	if !matched {
		log.Errorf("[Pair] base 格式错误 base:%v, quote:%v, more:%v", base, quote, more)
		return Pair{}
	}
	// 正则表达式判断 quote 是否数字和字母
	matched, err = regexp.MatchString("^[0-9a-zA-Z]+$", quote)
	if err != nil {
		log.Errorf("[Pair] quote 格式错误 base:%v, quote:%v, more:%v", base, quote, more)
		return Pair{}
	}
	if !matched {
		log.Errorf("[Pair] quote 格式错误 base:%v, quote:%v, more:%v", base, quote, more)
		return Pair{}
	}
	p := Pair{Base: strings.ToLower(base), Quote: strings.ToLower(quote), More: strings.ToLower(more)}
	_ = p.String()
	return p
}

func StringPairToPair(pair string) (Pair, error) {
	var temp []string
	if strings.Contains(pair, "/") {
		temp = strings.Split(pair, "/")
	} else {
		temp = strings.Split(pair, "_")
	}
	var p Pair
	if len(temp) == 3 {
		p = Pair{Base: strings.ToLower(temp[0]), Quote: strings.ToLower(temp[1]), More: strings.ToLower(temp[2])}
		_ = p.String()
		return p, nil
	} else if len(temp) == 2 {
		p = Pair{Base: strings.ToLower(temp[0]), Quote: strings.ToLower(temp[1]), More: ""}
		_ = p.String()
		return p, nil
	} else {
		return p, fmt.Errorf("[Pair] 格式错误 String:%v", pair)
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
// public
type Index struct {
	ID         atomic.Int64   // 更新序号 一般为时间戳
	IndexPrice atomic.Float64 // 指数价格
	MarkPrice  atomic.Float64 // 标记价格
}

func (t *Index) String() string {
	return fmt.Sprintf("%v %v", t.IndexPrice.Load(), t.MarkPrice.Load())
}

type IndexEvent struct {
	ID         float64 // 更新序号 一般为时间戳
	IndexPrice float64 // 指数价格
}
type MarkEvent struct {
	ID        float64 // 更新序号 一般为时间戳
	MarkPrice float64 // 指数价格
}

// 市场统计信息，mark price 等
type MarketStats struct {
	MarkPrice  float64
	IndexPrice float64
}

// 如果有某边缺失，数据不齐全，不更新ticker.
type Ticker struct {
	// ID  atomic.Int64 // 更新序号 一般为时间戳
	Seq Seq_T
	Bp  atomic.Float64
	Bq  atomic.Float64
	Ap  atomic.Float64
	Aq  atomic.Float64
	Mp  atomic.Float64
	// 根据推送ts和本地ts预估当前延迟 固定单位为ms 只能自己和过去自己比较
	// binance 现货推送只有updateId 这种数据无物理含义 无法计算delay 千万别用来计算
	// binance合约交易所推送的T E 是他们的本地clock ms 和我们本地时间比对 可以计算offset 用这个来评估延迟
	// 规定必须为撮合引擎时间戳 对应binance的T 如果交易所提供就填入 如果不提供就留空
	Delay atomic.Int64
	// 规定必须为推送事件时间戳 对应binance的E 正常情况下比T数值更大 大部分交易所不提供此字段 留空即可
	DelayE atomic.Int64
}

// 性能不佳，只在测试阶段使用
func (x *Ticker) MarshalJSONCustom() ([]byte, error) {
	bytesBuilder := bytes.NewBufferString("{")
	seq, err := json.Marshal(x.Seq)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(seq))
	bytesBuilder.WriteString("\"Seq\":")
	bytesBuilder.Write(seq)
	bytesBuilder.WriteString(",")
	bytesBuilder.WriteString(fmt.Sprintf("\"Ap\":%f,", x.Ap.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"Aq\":%f,", x.Aq.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"Bp\":%f,", x.Bp.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"Bq\":%f,", x.Bq.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"Mp\":%f,", x.Mp.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"Delay\":%d,", x.Delay.Load()))
	bytesBuilder.WriteString(fmt.Sprintf("\"DelayE\":%d", x.DelayE.Load()))
	bytesBuilder.WriteString("}")
	return bytesBuilder.Bytes(), nil
}

// 性能不佳，只在测试阶段使用
func (x *Ticker) UnmarshalJSONCustom(b []byte) error {
	var vt map[string]interface{}
	err := json.Unmarshal(b, &vt)
	if err != nil {
		return err
	}
	seq, ok := vt["Seq"]
	if !ok {
		return fmt.Errorf("seq not found")
	}
	seqBytes, err := json.Marshal(seq)
	if err != nil {
		return err
	}
	err = json.Unmarshal(seqBytes, &x.Seq)
	if err != nil {
		return err
	}
	bp, ok := vt["Bp"]
	if !ok {
		return fmt.Errorf("bp not found")
	}
	bpFloat, ok := bp.(float64)
	if !ok {
		return fmt.Errorf("bp is not a float64")
	}
	x.Bp.Store(bpFloat)

	bq, ok := vt["Bq"]
	if !ok {
		return fmt.Errorf("bq not found")
	}
	bqFloat, ok := bq.(float64)
	if !ok {
		return fmt.Errorf("bq is not a float64")
	}
	x.Bq.Store(bqFloat)

	ap, ok := vt["Ap"]
	if !ok {
		return fmt.Errorf("ap not found")
	}
	apFloat, ok := ap.(float64)
	if !ok {
		return fmt.Errorf("ap is not a float64")
	}
	x.Ap.Store(apFloat)

	aq, ok := vt["Aq"]
	if !ok {
		return fmt.Errorf("aq not found")
	}
	aqFloat, ok := aq.(float64)
	if !ok {
		return fmt.Errorf("aq is not a float64")
	}
	x.Aq.Store(aqFloat)

	mp, ok := vt["Mp"]
	if !ok {
		return fmt.Errorf("mp not found")
	}
	mpFloat, ok := mp.(float64)
	if !ok {
		return fmt.Errorf("mp is not a float64")
	}
	x.Mp.Store(mpFloat)

	delay, ok := vt["Delay"]
	if !ok {
		return fmt.Errorf("delay not found")
	}
	delayInt, ok := delay.(int64)
	if !ok {
		delayFloat, ok := delay.(float64)
		if !ok {
			return fmt.Errorf("delay is not a int64 or float64")
		}
		x.Delay.Store(int64(delayFloat))
	} else {
		x.Delay.Store(delayInt)
	}

	delayE, ok := vt["DelayE"]
	if !ok {
		return fmt.Errorf("delayE not found")
	}
	delayEInt, ok := delayE.(int64)
	if !ok {
		delayEFloat, ok := delayE.(float64)
		if !ok {
			return fmt.Errorf("delay is not a int64 or float64")
		}
		x.DelayE.Store(int64(delayEFloat))
	} else {
		x.DelayE.Store(delayEInt)
	}

	return nil
}

func (t *Ticker) Set(ap, aq, bp, bq float64) {
	t.Ap.Store(ap)
	t.Aq.Store(aq)
	t.Bp.Store(bp)
	t.Bq.Store(bq)
	if ap != 0 && bp != 0 {
		t.Mp.Store((ap + bp) * 0.5)
	} else if ap == 0 {
		t.Mp.Store(bp)
	} else if bp == 0 {
		t.Mp.Store(ap)
	}
}

func (t *Ticker) String() string {
	return fmt.Sprintf("Seq %s, bp %v, bq %v, ap %v, aq %v", t.Seq.String(), t.Bp.Load(), t.Bq.Load(), t.Ap.Load(), t.Aq.Load())
}

// MidPrice returns the middle of Bid and Ask.
func (t *Ticker) Price() float64 {
	if t.Ap.Load() != 0 && t.Bp.Load() != 0 {
		mp := (t.Bp.Load() + t.Ap.Load()) * 0.5
		return mp
	}
	if t.Ap.Load() == 0 {
		return t.Bp.Load()
	}
	return t.Ap.Load()
}

/*------------------------------------------------------------------------------------------------------------------*/
// private
type PositionEvent struct {
	// 这里的seq不对比，只是推送给上层用
	Seq             Seq_T `json:"S"`
	Pair            Pair  `json:"p,omitempty"`
	LongPos         fixed.Fixed
	LongAvg         float64
	ShortPos        fixed.Fixed
	ShortAvg        float64
	EventSourceType EventWayType
}

// SetPos 策略层把 pos 指针放进来 更新仓位 返回 true 表示成功 返回 false 表示 pair 不匹配
func (t *PositionEvent) SetPos(pos *Pos) bool {
	pos.Lock.Lock()
	defer pos.Lock.Unlock()
	if pos.Pair.Equal(t.Pair) {
		pos.LongPos = t.LongPos
		pos.LongAvg = t.LongAvg
		pos.ShortPos = t.ShortPos
		pos.ShortAvg = t.ShortAvg
		return true
	} else {
		return false
	}
}

func (t *PositionEvent) String() string {
	return fmt.Sprintf("Seq %s, Pair %s, LongPos %s, LongAvg %f, ShortPos %s, ShortAvg %f, EventWayType %d",
		t.Seq.String(), t.Pair,
		t.LongPos.String(), t.LongAvg,
		t.ShortPos.String(), t.ShortAvg,
		t.EventSourceType,
	)
}

// 仓位 兼容双向单向持仓模式
// easyjson:json
type Pos struct {
	Lock        sync.RWMutex `json:"-"`
	Seq         Seq_T        `json:"S"`
	Time        int64        `json:"tm,omitempty"`  // 最近一次更新时间 单位ms
	Pair        Pair         `json:"p,omitempty"`   // 交易对
	LongPos     fixed.Fixed  `json:"l,omitempty"`   // 多头量
	LongAvg     float64      `json:"la,omitempty"`  // 多头价格
	LongValue   float64      `json:"lv,omitempty"`  // 多头持仓价值
	ShortPos    fixed.Fixed  `json:"s,omitempty"`   // 空头量, 正数表示
	ShortAvg    float64      `json:"sa,omitempty"`  // 空头价格
	ShortValue  float64      `json:"sv,omitempty"`  // 空头持仓价值
	TradeNum    int64        `json:"t,omitempty"`   // 总成交次数
	TradeVol    float64      `json:"T,omitempty"`   // 总成交金额
	Profit      float64      `json:"P,omitempty"`   // 推算利润
	AHT         int64        `json:"aht,omitempty"` // 平均持仓时间
	OpenTs      int64        `json:"ots,omitempty"` // 建仓时间
	MaxDrawdown float64      `json:"md,omitempty"`  // 持仓期间最大回撤 每次 持仓value回归0 的时候 表示上一次的仓位已经清空 要重置为 0
}

func (p *Pos) SetReturnEvent(longPos, shortPos fixed.Fixed, longAvg, shortAvg float64) PositionEvent {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.LongAvg = longAvg
	p.ShortAvg = shortAvg
	p.LongPos = longPos
	p.ShortPos = shortPos
	return p.ToPositionEvent()
}

func (p *Pos) SetAvgReturnEvent(longAvg, shortAvg float64) PositionEvent {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.LongAvg = longAvg
	p.ShortAvg = shortAvg
	return p.ToPositionEvent()
}
func (p *Pos) Copy(in *Pos) {
	p.Seq = in.Seq
	p.Pair = in.Pair
	p.LongPos = in.LongPos
	p.LongAvg = in.LongAvg
	p.ShortPos = in.ShortPos
	p.ShortAvg = in.ShortAvg
}
func (p *Pos) ToPositionEvent() (pos PositionEvent) {
	pos.Seq = p.Seq
	pos.Pair = p.Pair
	pos.LongPos = p.LongPos
	pos.LongAvg = p.LongAvg
	pos.ShortPos = p.ShortPos
	pos.ShortAvg = p.ShortAvg
	return
}
func (p *Pos) ToPositionSums() []PositionSum {
	resp := make([]PositionSum, 0, 2)
	if p.LongPos.GreaterThan(fixed.ZERO) {
		resp = append(resp, PositionSum{
			Name:        p.Pair.String(),
			Amount:      p.LongPos.Float(),
			AvailAmount: p.LongPos.Float(),
			Ave:         p.LongAvg,
			Side:        PosSideLong,
		})
	}
	if p.ShortPos.GreaterThan(fixed.ZERO) {
		resp = append(resp, PositionSum{
			Name:        p.Pair.String(),
			Amount:      p.ShortPos.Float(),
			AvailAmount: p.ShortPos.Float(),
			Ave:         p.ShortAvg,
			Side:        PosSideShort,
		})
	}
	return resp
}
func (p *Pos) String() string {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	return fmt.Sprintf("Seq %s, 仓位 多 A:%s P:%f 空 A:%s P:%f", p.Seq.String(), p.LongPos.String(), p.LongAvg, p.ShortPos.String(), p.ShortAvg)
}

// 不适用，去除了 ID kc@2023-11-19
// func (p *Pos) GetId() int64 {
// 	p.Lock.RLock()
// 	defer p.Lock.RUnlock()
// 	return p.ID
// }

func (p *Pos) GetTradeNum() int64 {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	return p.TradeNum
}

func (p *Pos) GetTradeVol() float64 {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	return p.TradeVol
}

func (p *Pos) GetProfit() float64 {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	return p.Profit
}

func (p *Pos) NetPos() (fixed.Fixed, fixed.Fixed) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	size := p.LongPos.Sub(p.ShortPos)
	ave := fixed.ZERO
	if size.GreaterThan(fixed.ZERO) {
		ave = fixed.NewF(p.LongAvg)
	} else if size.LessThan(fixed.ZERO) {
		ave = fixed.NewF(p.ShortAvg)
	}
	return size, ave
}

// UpdateMaxDrawDown 更新本次持仓期间最大回撤
func (p *Pos) UpdateMaxDrawDown(mp float64) float64 {
	// 加锁
	p.Lock.Lock()
	defer p.Lock.Unlock()
	var drawdown float64
	if p.LongPos.GreaterThan(fixed.ZERO) {
		drawdown = (mp - p.LongAvg) / p.LongAvg // 负值 表示 回撤
		if drawdown < p.MaxDrawdown {
			p.MaxDrawdown = drawdown
		}
	}
	if p.ShortPos.GreaterThan(fixed.ZERO) {
		drawdown = (p.ShortAvg - mp) / mp
		if drawdown < p.MaxDrawdown {
			p.MaxDrawdown = drawdown
		}
	}
	return p.MaxDrawdown
}

// Update 传入信息 更新仓位
// 同步更新成交额 累计利润
// 考虑手续费影响 feeRate 为0表示不考虑手续费 为0.001表示被收取千分之一 为-0.0001表示被返还万分之一
func (p *Pos) UpdateWithFee(side OrderSide, filled fixed.Fixed, filledPrice float64, allowComb bool, time int64, feeRate float64) float64 {
	// 加锁
	p.Lock.Lock()
	defer p.Lock.Unlock()
	// 开始计算
	var pnl float64
	p.Time = time
	p.TradeNum++
	// 区分数量单位
	tradeValue := filled.Float() * filledPrice
	p.TradeVol += tradeValue
	if DEBUGMODE {
		log.Debugf("更新仓位 side:%v A:%v P:%v", side, filled, filledPrice)
		log.Debugf("计算前 多 A:%s P:%f 空 A:%s P:%f", p.LongPos.String(), p.LongAvg, p.ShortPos.String(), p.ShortAvg)
	}
	if filledPrice == 0 {
		log.Errorf("Pos.Update绝对禁止出现filledPrice为0的情况")
	}
	// 如果是单向持仓模式 请务必传入true 进行仓位方向合并
	if allowComb {
		// 单向持仓模式的计算
		switch side {
		// buy
		case OrderSideKD, OrderSidePK:
			// 计算仓位
			if p.ShortPos.GreaterThan(fixed.ZERO) {
				// 需要先处理空头仓位
				if p.ShortPos.GreaterThanOrEqual(filled) {
					// 减少空仓 空仓还有剩余
					pnl = tradeValue * (p.ShortAvg - filledPrice) / filledPrice
					p.ShortPos = p.ShortPos.Sub(filled)
					if p.ShortPos.Equal(fixed.ZERO) {
						p.ShortAvg = 0.0
						p.ShortValue = 0.0
						p.MaxDrawdown = 0.0
					} else {
						p.ShortValue -= filled.Float() * p.ShortAvg
					}
				} else {
					diff := filled.Sub(p.ShortPos)
					diffValue := diff.Float() * filledPrice
					// 减少空仓 到零
					pnl = p.ShortPos.Float() * filledPrice * (p.ShortAvg - filledPrice) / filledPrice
					p.ShortPos = fixed.ZERO
					p.ShortAvg = 0.0
					p.ShortValue = 0.0
					p.MaxDrawdown = 0.0
					// 增加多仓
					p.LongPos = p.LongPos.Add(diff)
					p.LongAvg = (p.LongValue + diffValue) / p.LongPos.Float()
					p.LongValue += diffValue
				}
			} else {
				// 直接增加多仓
				p.LongPos = p.LongPos.Add(filled)
				p.LongAvg = (p.LongValue + tradeValue) / p.LongPos.Float()
				p.LongValue += tradeValue
			}
		// sell
		case OrderSideKK, OrderSidePD:
			// 计算仓位
			if p.LongPos.GreaterThan(fixed.ZERO) {
				// 需要先处理多头仓位
				if p.LongPos.GreaterThanOrEqual(filled) {
					// 减少多仓 多仓可能有剩余
					pnl = tradeValue * (filledPrice - p.LongAvg) / filledPrice
					p.LongPos = p.LongPos.Sub(filled)
					if p.LongPos.Equal(fixed.ZERO) {
						p.LongAvg = 0.0
						p.LongValue = 0.0
						p.MaxDrawdown = 0.0
					} else {
						p.LongValue -= filled.Float() * p.LongAvg
					}
				} else {
					diff := filled.Sub(p.LongPos)
					diffValue := diff.Float() * filledPrice
					// 先减少多仓 到零
					pnl = p.LongPos.Float() * filledPrice * (filledPrice - p.LongAvg) / filledPrice
					p.LongPos = fixed.ZERO
					p.LongAvg = 0.0
					p.LongValue = 0.0
					p.MaxDrawdown = 0.0
					// 再增加空仓
					p.ShortPos = p.ShortPos.Add(diff)
					p.ShortAvg = (p.ShortValue + diffValue) / p.ShortPos.Float()
					p.ShortValue += diffValue
				}
			} else {
				// 直接增加空头仓位
				p.ShortPos = p.ShortPos.Add(filled)
				p.ShortAvg = (p.ShortValue + tradeValue) / p.ShortPos.Float()
				p.ShortValue += tradeValue
			}
		}
	} else {
		// 双向持仓模式的计算
		switch side {
		case OrderSideKD:
			p.LongPos = p.LongPos.Add(filled)
			p.LongAvg = (p.LongValue + tradeValue) / p.LongPos.Float()
			p.LongValue += tradeValue
		case OrderSideKK:
			p.ShortPos = p.ShortPos.Add(filled)
			p.ShortAvg = (p.ShortValue + tradeValue) / p.ShortPos.Float()
			p.ShortValue += tradeValue
		case OrderSidePD:
			pnl = tradeValue * (filledPrice - p.LongAvg) / filledPrice
			p.LongPos = p.LongPos.Sub(filled)
			if p.LongPos.LessThanOrEqual(fixed.ZERO) {
				p.LongPos = fixed.ZERO
				p.LongAvg = 0.0
				p.LongValue = 0.0
				p.MaxDrawdown = 0.0
			} else {
				p.LongValue -= filled.Float() * p.LongAvg
			}
		case OrderSidePK:
			pnl = tradeValue * (p.ShortAvg - filledPrice) / filledPrice
			p.ShortPos = p.ShortPos.Sub(filled)
			if p.ShortPos.LessThanOrEqual(fixed.ZERO) {
				p.ShortPos = fixed.ZERO
				p.ShortAvg = 0.0
				p.ShortValue = 0.0
				p.MaxDrawdown = 0.0
			} else {
				p.ShortValue -= filled.Float() * p.ShortAvg
			}
		}
	}
	// 计算aht
	switch side {
	case OrderSideKD, OrderSideKK:
		p.OpenTs = time
	case OrderSidePD, OrderSidePK:
		if p.OpenTs > 0 {
			diff := time - p.OpenTs
			p.AHT = (p.AHT*9 + diff) / 10
			p.OpenTs = 0
		}
	}
	// 计算费前累计利润
	p.Profit += pnl - tradeValue*feeRate
	if DEBUGMODE {
		log.Debugf("计算后 多 A:%s P:%f 空 A:%s P:%f", p.LongPos.String(), p.LongAvg, p.ShortPos.String(), p.ShortAvg)
	}
	return pnl
}

// Update 传入信息 更新仓位
// 同步更新成交额 累计利润
// 没有考虑手续费影响
func (p *Pos) Update(side OrderSide, filled fixed.Fixed, filledPrice float64, allowComb bool, time int64) float64 {
	// 加锁
	p.Lock.Lock()
	defer p.Lock.Unlock()
	// 开始计算
	var pnl float64
	p.Time = time
	p.TradeNum++
	// 区分数量单位
	tradeValue := filled.Float() * filledPrice
	p.TradeVol += tradeValue
	if DEBUGMODE {
		log.Debugf("更新仓位 side:%v A:%v P:%v", side, filled, filledPrice)
		log.Debugf("计算前 多 A:%s P:%f 空 A:%s P:%f", p.LongPos.String(), p.LongAvg, p.ShortPos.String(), p.ShortAvg)
	}
	if filledPrice == 0 {
		log.Errorf("Pos.Update绝对禁止出现filledPrice为0的情况")
	}
	// 如果是单向持仓模式 请务必传入true 进行仓位方向合并
	if allowComb {
		// 单向持仓模式的计算
		switch side {
		// buy
		case OrderSideKD, OrderSidePK:
			// 计算仓位
			if p.ShortPos.GreaterThan(fixed.ZERO) {
				// 需要先处理空头仓位
				if p.ShortPos.GreaterThanOrEqual(filled) {
					// 减少空仓 空仓还有剩余
					pnl = tradeValue * (p.ShortAvg - filledPrice) / filledPrice
					p.ShortPos = p.ShortPos.Sub(filled)
					if p.ShortPos.Equal(fixed.ZERO) {
						p.ShortAvg = 0.0
						p.ShortValue = 0.0
						p.MaxDrawdown = 0.0
					} else {
						p.ShortValue -= filled.Float() * p.ShortAvg
					}
				} else {
					diff := filled.Sub(p.ShortPos)
					diffValue := diff.Float() * filledPrice
					// 减少空仓 到零
					pnl = p.ShortPos.Float() * filledPrice * (p.ShortAvg - filledPrice) / filledPrice
					p.ShortPos = fixed.ZERO
					p.ShortAvg = 0.0
					p.ShortValue = 0.0
					p.MaxDrawdown = 0.0
					// 增加多仓
					p.LongPos = p.LongPos.Add(diff)
					p.LongAvg = (p.LongValue + diffValue) / p.LongPos.Float()
					p.LongValue += diffValue
				}
			} else {
				// 直接增加多仓
				p.LongPos = p.LongPos.Add(filled)
				p.LongAvg = (p.LongValue + tradeValue) / p.LongPos.Float()
				p.LongValue += tradeValue
			}
		// sell
		case OrderSideKK, OrderSidePD:
			// 计算仓位
			if p.LongPos.GreaterThan(fixed.ZERO) {
				// 需要先处理多头仓位
				if p.LongPos.GreaterThanOrEqual(filled) {
					// 减少多仓 多仓可能有剩余
					pnl = tradeValue * (filledPrice - p.LongAvg) / filledPrice
					p.LongPos = p.LongPos.Sub(filled)
					if p.LongPos.Equal(fixed.ZERO) {
						p.LongAvg = 0.0
						p.LongValue = 0.0
						p.MaxDrawdown = 0.0
					} else {
						p.LongValue -= filled.Float() * p.LongAvg
					}
				} else {
					diff := filled.Sub(p.LongPos)
					diffValue := diff.Float() * filledPrice
					// 先减少多仓 到零
					pnl = p.LongPos.Float() * filledPrice * (filledPrice - p.LongAvg) / filledPrice
					p.LongPos = fixed.ZERO
					p.LongAvg = 0.0
					p.LongValue = 0.0
					p.MaxDrawdown = 0.0
					// 再增加空仓
					p.ShortPos = p.ShortPos.Add(diff)
					p.ShortAvg = (p.ShortValue + diffValue) / p.ShortPos.Float()
					p.ShortValue += diffValue
				}
			} else {
				// 直接增加空头仓位
				p.ShortPos = p.ShortPos.Add(filled)
				p.ShortAvg = (p.ShortValue + tradeValue) / p.ShortPos.Float()
				p.ShortValue += tradeValue
			}
		}
	} else {
		// 双向持仓模式的计算
		switch side {
		case OrderSideKD:
			p.LongPos = p.LongPos.Add(filled)
			p.LongAvg = (p.LongValue + tradeValue) / p.LongPos.Float()
			p.LongValue += tradeValue
		case OrderSideKK:
			p.ShortPos = p.ShortPos.Add(filled)
			p.ShortAvg = (p.ShortValue + tradeValue) / p.ShortPos.Float()
			p.ShortValue += tradeValue
		case OrderSidePD:
			pnl = tradeValue * (filledPrice - p.LongAvg) / filledPrice
			p.LongPos = p.LongPos.Sub(filled)
			if p.LongPos.LessThanOrEqual(fixed.ZERO) {
				p.LongPos = fixed.ZERO
				p.LongAvg = 0.0
				p.LongValue = 0.0
				p.MaxDrawdown = 0.0
			} else {
				p.LongValue -= filled.Float() * p.LongAvg
			}
		case OrderSidePK:
			pnl = tradeValue * (p.ShortAvg - filledPrice) / filledPrice
			p.ShortPos = p.ShortPos.Sub(filled)
			if p.ShortPos.LessThanOrEqual(fixed.ZERO) {
				p.ShortPos = fixed.ZERO
				p.ShortAvg = 0.0
				p.ShortValue = 0.0
				p.MaxDrawdown = 0.0
			} else {
				p.ShortValue -= filled.Float() * p.ShortAvg
			}
		}
	}
	// 计算aht
	switch side {
	case OrderSideKD, OrderSideKK:
		p.OpenTs = time
	case OrderSidePD, OrderSidePK:
		if p.OpenTs > 0 {
			diff := time - p.OpenTs
			p.AHT = (p.AHT*9 + diff) / 10
			p.OpenTs = 0
		}
	}
	// 计算费前累计利润
	p.Profit += pnl
	if DEBUGMODE {
		log.Debugf("计算后 多 A:%s P:%f 空 A:%s P:%f", p.LongPos.String(), p.LongAvg, p.ShortPos.String(), p.ShortAvg)
	}
	return pnl
}

// 重置仓位
func (p *Pos) Reset() {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.LongPos = fixed.ZERO
	p.LongAvg = 0.0
	p.LongValue = 0.0
	p.ShortPos = fixed.ZERO
	p.ShortAvg = 0.0
	p.ShortValue = 0.0
	p.MaxDrawdown = 0.0
	p.OpenTs = 0
}

func (p *Pos) ResetReturnEvent() PositionEvent {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.LongPos = fixed.ZERO
	p.LongAvg = 0.0
	p.LongValue = 0.0
	p.ShortPos = fixed.ZERO
	p.ShortAvg = 0.0
	p.ShortValue = 0.0
	p.MaxDrawdown = 0.0
	p.OpenTs = 0
	return p.ToPositionEvent()
}

// 以加锁调用这个
func (p *Pos) ResetLocked() {
	p.LongPos = fixed.ZERO
	p.LongAvg = 0.0
	p.LongValue = 0.0
	p.ShortPos = fixed.ZERO
	p.ShortAvg = 0.0
	p.ShortValue = 0.0
	p.MaxDrawdown = 0.0
}

type EventWayType int

const (
	EventWayWs EventWayType = iota
	EventWayRs
)

/*------------------------------------------------------------------------------------------------------------------*/
type FieldsSet_T int

const (
	EquityEventField_TotalWithUpl    FieldsSet_T = 1 << iota
	EquityEventField_TotalWithoutUpl FieldsSet_T = 1 << iota
	EquityEventField_Avail           FieldsSet_T = 1 << iota
	EquityEventField_Upl             FieldsSet_T = 1 << iota
)

func (f FieldsSet_T) ContainsAll(field ...FieldsSet_T) bool {
	for _, item := range field {
		if f&item != item {
			return false
		}
	}
	return true
}
func (f FieldsSet_T) ContainsOne(field ...FieldsSet_T) bool {
	for _, item := range field {
		if f&item == item {
			return true
		}
	}
	return false
}

// 资产
type EquityEvent struct {
	Seq  Seq_T  `json:"S"`
	Name string //资产名字，小写，如btc
	// 有些交易所的总额包含Upl，但不给出Upl具体值，所以要分包含或不包含
	TotalWithUpl    float64 // 包含浮盈浮亏
	TotalWithoutUpl float64 // 不包含浮盈浮亏
	Avail           float64
	Upl             float64 // 浮盈浮亏
	//  todo 如果每个所在确定EventWay返回的equity field都一致，不随时间、仓位而缺失，可以考虑用初始化时一个函数表示。积累信息后确认 kc@2024-7-15
	FieldsSet FieldsSet_T // 哪些字段有意义, EquityEventField 的 bitset
	EventWay  EventWayType
}

func (e *EquityEvent) String() string {
	return fmt.Sprintf("Seq %s, Name %s, TotalWithUpl %v, TotalWithoutUpl %v, Avail %v Upl %v, FieldsSet %04b, EventWay %d",
		e.Seq.String(), e.Name, e.TotalWithUpl, e.TotalWithoutUpl, e.Avail, e.Upl, e.FieldsSet, e.EventWay)
}

// 对于合约交易 cash 代表你合约账户 该交易对的保证金数量 比如 btc_usdt binance_usdt_swap cash对应的就是usdt的数量
// 对于现货交易 cash 代表可用保证金数量 coin代表币的数量 比如 btc_usdt binance_spot cash对应该账号现货钱包的usdt数量 coin对应该账号现货钱包的btc的数量
// 这样设计的目的 Equity结构体可以兼容现货和合约交易
// easyjson:json
type Equity struct {
	Lock      sync.RWMutex `json:"-"`
	Seq       Seq_T        `json:"S"`
	CoinPrice float64      `json:"CP,omitempty"` // 币保证金的法币价格 不需要实时更新 只用于粗略计算coin的法币价值
	Coin      float64      `json:"C,omitempty"`  // 币
	CoinFree  float64      `json:"CF,omitempty"` // 币 可用
	Cash      float64      `json:"c,omitempty"`  // u 包含 浮盈浮亏
	CashFree  float64      `json:"cf,omitempty"` // u 可用
	CashUpl   float64      `json:"cu,omitempty"` // u 浮盈浮亏
	IsSet     bool         `json:"s,omitempty"`  // 是否已经设置值，在用作返回值时根据这个判断
	FieldsSet FieldsSet_T
}

func (e *Equity) Reset() {
	e.Lock.Lock()
	e.Coin = 0.0
	e.CoinFree = 0.0
	e.Cash = 0.0
	e.CashFree = 0.0
	e.CashUpl = 0.0
	e.Lock.Unlock()
}
func (e *Equity) ResetNoLock() {
	e.Coin = 0.0
	e.CoinFree = 0.0
	e.Cash = 0.0
	e.CashFree = 0.0
	e.CashUpl = 0.0
}

func (e *Equity) String() string {
	e.Lock.RLock()
	defer e.Lock.RUnlock()
	// 默认是6，扩大到18位
	return fmt.Sprintf("Seq %s, 资产 Cash %.2f Coin %.18f CashFree %.2f CoinFree %.18f CashUpnl %.2f", e.Seq.String(), e.Cash, e.Coin, e.CashFree, e.CoinFree, e.CashUpl)
}

// Update 传入信息 更新仓位
// 同步更新成交额 累计利润 todo 暂时没有考虑手续费影响和taker maker 分开统计
func (e *Equity) Update(side OrderSide, filled fixed.Fixed, filledPrice float64) {
	e.Lock.Lock()
	defer e.Lock.Unlock()

	tradeValue := filled.Float() * filledPrice

	if DEBUGMODE {
		log.Debugf("更新资金 side:%v A:%v P:%v", side, filled, filledPrice)
		log.Debugf("计算前 币:%f 可用币:%f  现金:%f 可用现金:%f", e.Coin, e.CoinFree, e.Cash, e.CashFree)
	}

	switch side {
	// buy
	case OrderSideKD, OrderSidePK:
		e.Coin = e.Coin + filled.Float()
		e.CoinFree = e.CoinFree + filled.Float()
		e.Cash = e.Cash - tradeValue
		e.CashFree = e.CashFree - tradeValue
	// sell
	case OrderSideKK, OrderSidePD:
		e.Coin = e.Coin - filled.Float()
		e.CoinFree = e.CoinFree - filled.Float()
		e.Cash = e.Cash + tradeValue
		e.CashFree = e.CashFree + tradeValue
	}
	// 费前
	if DEBUGMODE {
		log.Debugf("计算后 币:%f 可用币:%f  现金:%f 可用现金:%f", e.Coin, e.CoinFree, e.Cash, e.CashFree)
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
// 订单方向
type OrderSide int

const (
	OrderSideUnKnown OrderSide = iota // 用于部分所拿不到这个信息 依然需要填充字段
	OrderSideKD
	OrderSideKK
	OrderSidePD
	OrderSidePK
)

func StrToOrderSide(str string) OrderSide {
	switch str {
	case "KD":
		return OrderSideKD
	case "KK":
		return OrderSideKK
	case "PD":
		return OrderSidePD
	case "PK":
		return OrderSidePK
	default:
		return OrderSideUnKnown
	}
}

func (o OrderSide) String() string {
	switch o {
	case OrderSideKD:
		return "KD"
	case OrderSideKK:
		return "KK"
	case OrderSidePD:
		return "PD"
	case OrderSidePK:
		return "PK"
	default:
		return "OrderSide_Unknown"
	}
}
func (s OrderSide) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

/*------------------------------------------------------------------------------------------------------------------*/
// 订单状态
type OrderState int

const (
	OrderStateAll OrderState = iota
	OrderStateSubmit
	OrderStatePending
	OrderStateCancelling
	OrderStateFinished
)

func (o OrderState) String() string {
	switch o {
	case OrderStateSubmit:
		return "OrderState_Submit"
	case OrderStatePending:
		return "OrderStates_Pending"
	case OrderStateCancelling:
		return "OrderStates_Cancelling"
	default:
		return "OrderStatus_Unknown"
	}
}
func (s OrderState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

/*------------------------------------------------------------------------------------------------------------------*/
type OrderErrorType int

const (
	OrderErrorTypeNone               OrderErrorType = iota
	OrderErrorTypeNotAllowOpen                      // 不允许开仓，但可能允许平仓
	OrderErrorTypeNotAllowReduceOnly                // 部分所部分账户，不允许reduce only
)

func (o OrderErrorType) String() string {
	switch o {
	case OrderErrorTypeNone:
		return "OrderErrorType_None"
	case OrderErrorTypeNotAllowOpen:
		return "OrderErrorType_NotAllowOpen"
	case OrderErrorTypeNotAllowReduceOnly:
		return "OrderErrorType_NotAllowReduceOnly"
	default:
		return "OrderErrorType_Unknown"
	}
}

func (s OrderErrorType) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// 订单事件类型
type OrderEventType int

const (
	OrderEventTypeNEW OrderEventType = 1 + iota // 创建新订单
	/**
	remove 会尝试计算仓位
	error 直接移除本地缓存
	如果确定100%这个订单被拒绝 可以走onerror 节省性能, 千万不能出现onerror了 订单后来又成交了这种情况
	*/
	OrderEventTypeREMOVE    // 移除订单缓存 来自正常交易产生的cancel和filled 并触发产生新signal
	OrderEventTypeERROR     // 移除订单缓存 来自下单失败 不触发产生新signal
	OrderEventTypePARTIAL   // 部分成交 通知策略层存在部分成交的情况
	OrderEventTypeAmendSucc // 改单成功
	OrderEventTypeAmendFail // 改单失败
	OrderEventTypeNotFound
)

func (o OrderEventType) String() string {
	switch o {
	case OrderEventTypeNEW:
		return "NEW"
	case OrderEventTypeREMOVE:
		return "REMOVE"
	case OrderEventTypePARTIAL:
		return "PARTIAL"
	case OrderEventTypeERROR:
		return "ERROR"
	case OrderEventTypeNotFound:
		return "NotFound"
	case OrderEventTypeAmendSucc:
		return "AmendSucc"
	case OrderEventTypeAmendFail:
		return "AmendFail"
	default:
		return "Unknown OrderEventType"
	}
}
func (s OrderEventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

/*------------------------------------------------------------------------------------------------------------------*/

type FilledType int

const (
	FilledTypeCumSum = 0 + iota // 累计成交
	FilledTypeThis              // 本次成交
)

type OrderEventSourceType int

// 订单事件来源
const (
	OrderEventSourceLiquid   OrderEventSourceType = iota // 常规清算系统推送
	OrderEventSourceMatch                                // 撮合引擎推送
	OrderEventSourceCustom_1                             // 自定义来源
	OrderEventSourceCustom_2
)

func (t OrderEventSourceType) String() string {
	switch t {
	case OrderEventSourceLiquid:
		return "Liquid"
	case OrderEventSourceMatch:
		return "Match"
	case OrderEventSourceCustom_1:
		return "Custom_1"
	case OrderEventSourceCustom_2:
		return "Custom_2"
	}
	return "unknow"
}
func (s OrderEventSourceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

/*------------------------------------------------------------------------------------------------------------------*/

// 订单事件
// easyjson:json
type OrderEvent struct {
	Pair            Pair
	ID              int64          `json:"I"` // 触发order的seq
	Type            OrderEventType `json:"t,omitempty"`
	ClientID        string         `json:"C,omitempty"`
	OrderID         string         `json:"O,omitempty"`
	OrderType       OrderType      `json:"ot,omitempty"` // 订单类型 Limit Market PostOnly等等 交易所提供的话尽量补充
	OrderSide       OrderSide      `json:"os,omitempty"` // 部分交易所不支持 策略层面优先使用本地缓存的order的side 其次再使用本字段
	Amount          fixed.Fixed    `json:"a,omitempty"`  // 原始单量
	Price           float64        `json:"p,omitempty"`  // 原始价格
	Filled          fixed.Fixed    `json:"f,omitempty"`  // 仅允许出现>=0的数字(消除合约乘数后的原始数量，参照okx usdt swap)
	FilledPrice     float64        `json:"fp,omitempty"`
	FilledThis      fixed.Fixed    `json:"fth,omitempty"` // 本次成交量，部分所是空
	FilledPriceThis float64        `json:"fpt,omitempty"` // 本次成交价格，部分所可能是空
	// deprecated
	FilledType       FilledType           `json:"ft,omitempty"` // 成交类型
	CashFee          fixed.Fixed          `json:"c,omitempty"`  // 手续费正为扣手续费 负为rebate
	CoinFee          fixed.Fixed          `json:"cf,omitempty"` // 手续费正为扣手续费 负为rebate
	ErrorReason      string               `json:"er,omitempty"` // 错误原因
	ErrorType        OrderErrorType       `json:"et,omitempty"`
	ReceivedTsNs     int64                `json:"-"` // 本地收到时的时间戳
	OrderEventSource OrderEventSourceType `json:"-"`
	Extra            []any                `json:"-"`
}

func (o *OrderEvent) String() string {
	return fmt.Sprintf("订单事件[%s] pair %s, cid:%s oid:%s tsns:%d type:%s side:%s filled:%s@%f thisFilled:%s@%f src:%s err:%s",
		o.Type.String(), o.Pair.String(), o.ClientID, o.OrderID, o.ReceivedTsNs, o.OrderType, o.OrderSide, o.Filled.String(), o.FilledPrice, o.FilledThis.String(), o.FilledPriceThis, o.OrderEventSource.String(), o.ErrorReason)
}

type TradeEvent struct {
	TakerSide     OrderSide
	TimeMatched   uint64
	TimePushEvent uint64
	Amount        float64
	Price         float64
}

func (t *TradeEvent) String() string {
	return fmt.Sprintf("成交事件[%s] side:%s amount:%f price:%f matched:%d push:%d", t.TakerSide, t.TakerSide, t.Amount, t.Price, t.TimeMatched, t.TimePushEvent)
}

/*------------------------------------------------------------------------------------------------------------------*/
// 订单
type Order struct {
	ClientID   string      // 客户定义的id
	OrderID    string      // 交易所的id
	Price      float64     // 下单价格
	Amount     fixed.Fixed // 数量
	OrderSide  OrderSide   // 订单方向
	OrderType  OrderType   // 订单类型
	OrderState OrderState  // 订单当前状态
	// 订单事件时间 1、创建订单的时候，2、发起查单的时候
	EventTime          int64 // 单位毫秒
	PlaceConfirmedTime int64 // 单位毫秒
	//订单撤单时间 1、发起撤单的时候
	CancelTime   int64       // 单位毫秒
	CancelNum    int64       // 撤单次数
	CheckNum     int64       // 查单次数
	Text         string      // 内部订单备注
	InnerType    int         // 内部订单类型
	OppoPrice    float64     // 下单时刻的对手价格 用于计算市价滑点
	TradedAmount fixed.Fixed // 已成数量， 用于partialfill
	PartialPrice float64     // 已成交价格， 用于partialfill
}

func (o *Order) String() string {
	value := "M"
	if o.Price > 0 {
		value = fmt.Sprintf("%.2f", o.Price*o.Amount.Float())
	}
	return fmt.Sprintf("订单信息[%s] [%s] %s %s %s P:%.10f A:%v V:%v EventT:%d CancelT:%d CancelN:%d CheckN:%d 对价:%f 备注:%s",
		o.ClientID, o.OrderID,
		o.OrderSide.String(), o.OrderState.String(), o.OrderType.String(),
		o.Price, o.Amount, value,
		TsToHumanMillis(o.EventTime), TsToHumanMillis(o.CancelTime), o.CancelNum, o.CheckNum,
		o.OppoPrice, o.Text,
	)
}

type AmountUnitType_T int

/*
* 量的单位
Base 是基础币，计价币面值等于 Amount * Multi * price(mark price or indexprice or others)
Quote 是计价币，计价币面值等于 Amount * Multi
Piece 是张，再用UnderlyingBase和UnderlyingQuote区分。例如一个合约的量单位是Piece + UnderlyingBase，bid quantity是3，则对应的计价币价值= 3 * Amount * Multi * price
*/
const (
	AmountUnitType_Base AmountUnitType_T = iota + 0
	AmountUnitType_Quote
	// AmountUnitType_Piece // 张
)

type UnderlyingType_T int

const (
	UnderlyingBase UnderlyingType_T = iota + 0
	UnderlyingQuote
)

// 容易变化的信息
type LabileExchangeInfo struct {
	Pair           Pair      `json:"Pair"`           // 交易对名字 内部规则
	Symbol         string    `json:"Symbol"`         // 交易对名字 交易所规则
	SettedLeverage int       `json:"SettedLeverage"` // 当前设置的杠杆
	RiskLimit      RiskLimit `json:"RiskLimit"`      // 当前设定下的最大开仓限额
}

// 不容易变化的信息
/*------------------------------------------------------------------------------------------------------------------*/
// 交易规则 可能会有竞争
type ExchangeInfo struct {
	// Lock     sync.Mutex  // 动态更新交易对规则 todo 将来此处改为用指针锁
	Pair         Pair        `json:"Pair"`         // 交易对名字 内部规则
	Symbol       string      `json:"Symbol"`       // 交易对名字 交易所规则
	Status       bool        `json:"Status"`       // 是否允许交易
	TickSize     float64     `json:"TickSize"`     // 价格精度
	StepSize     float64     `json:"StepSize"`     // 数量精度
	ContractSize float64     `json:"ContractSize"` // 合约张数精度，有些交易所例如okx支持0.x张合约下单
	Multi        fixed.Fixed `json:"Multi"`        // 合约乘数  multi * 数量(单位张) =  数量(单位币)  数量(单位币) / multi = 数量(单位张)
	// 下面这4个限制 既有以币为单位的 也有以u为单位的 理论上可以兼容任何下单限制规则
	// 杠杆 => 币 u 具体只需要在beforeTrade里面去适配就可以了
	MaxOrderAmount      fixed.Fixed `json:"MaxOrderAmount"` // 最大下单数量
	MaxOrderValue       fixed.Fixed `json:"MaxOrderValue"`  // 最大下单金额
	MaxLimitOrderAmount fixed.Fixed // 最大下单数量, 有些所例如ok分订单类型限制
	MaxLimitOrderValue  fixed.Fixed // 最大下单金额
	MinOrderAmount      fixed.Fixed `json:"MinOrderAmount"` // 最小下单数量
	MinOrderValue       fixed.Fixed `json:"MinOrderValue"`  // 最小下单金额

	// 下面2个限制 用来控制持仓大小
	MaxPosAmount fixed.Fixed `json:"MaxPosAmount"` // 最大持仓数量
	MaxPosValue  fixed.Fixed `json:"MaxPosValue"`  // 最大持仓金额

	MaxLeverage int `json:"MaxLeverage"` // 底层设置的最大杠杆 策略层实际使用的杠杆 不要超过这个值
	RiskLimit   RiskLimit
	// todo 将来在这里加入限价规则

	// l2 所需信息
	StarkExQuoteAssetId    string //链上币种id, 16进制0x开头， like 0x13434
	StarkExBaseAssetId     string //链上币种id
	StarkExQuoteResolution int
	StarkExBaseResolution  int

	// 合约单位信息
	AmountUnitType AmountUnitType_T
	UnderlyingType UnderlyingType_T

	Extra  string `json:"extra"` // 额外辅助信息
	Extras []interface{}
}

const EXIT_MSG_PRICES_DEV_TOO_LARGE = "Prices deviation is too large, need to exit"

// CheckReady 检查交易所信息是否完整
func (e *ExchangeInfo) CheckReady() (bool, string) {
	flag := true
	err := "无异常"
	if e.Pair.Base == "" {
		return false, "交易对缺少Base"
	}
	if e.Pair.Quote == "" {
		return false, "交易对缺少Quote"
	}
	if e.Symbol == "" {
		return false, "交易对缺少Symbol"
	}
	if e.TickSize == 0.0 {
		return false, "交易对缺少TickSize"
	}
	if e.StepSize == 0.0 {
		return false, "交易对缺少StepSize"
	}
	//if e.Multi.Equal(fixed.ZERO) {
	//	return false, "交易对缺少Multi"
	//}
	if e.MaxOrderAmount.Equal(fixed.ZERO) {
		return false, "交易对缺少MaxOrderAmount"
	}
	if e.MaxOrderValue.Equal(fixed.ZERO) {
		return false, "交易对缺少MaxOrderValue"
	}
	//if e.MinOrderAmount.Equal(fixed.ZERO) {
	//	return false, "交易对缺少MinOrderAmount"
	//}
	//if e.MinOrderValue.Equal(fixed.ZERO) {
	//	return false, "交易对缺少MinOrderValue"
	//}
	if e.MaxPosAmount.Equal(fixed.ZERO) {
		return false, "交易对缺少MaxPosAmount"
	}
	if e.MaxPosValue.Equal(fixed.ZERO) {
		return false, "交易对缺少MaxPosValue"
	}
	//if e.MaxLeverage == 0 {
	//	return false, "交易对缺少MaxLeverage"
	//}
	//if e.RiskLimit.Underlying == "" {
	//	return false, "交易对缺少RiskLimit.Underlying"
	//}
	//if e.RiskLimit.Amount == 0.0 {
	//	return false, "交易对缺少RiskLimit.Amount"
	//}
	return flag, err
}

// 最大持仓限额
type RiskLimit struct {
	Underlying string // 持仓限额的计算单位，有些是稳定币、法币、加密币. 小写
	Amount     float64
}

func (e ExchangeInfo) String() string {
	return fmt.Sprintf("品种%s 交易对%v 状态%v 价格精度%v 数量精度%v 乘数%v 最大下单量%v "+
		"最小下单量%v 最大下单金额%v 最小下单金额%v 最大持仓量%v 最大持仓金额%v 最大杠杆%v Risklimit %v",
		e.Pair.ToString(), e.Symbol, e.Status, e.TickSize, e.StepSize, e.Multi, e.MaxOrderAmount.Float(), e.MinOrderAmount.Float(),
		e.MaxOrderValue.Float(), e.MinOrderValue.Float(), e.MaxPosAmount.Float(), e.MaxPosValue.Float(), e.MaxLeverage, e.RiskLimit)
}

/*------------------------------------------------------------------------------------------------------------------*/
// 订单类型
type OrderType int

const (
	OrderTypeLimit OrderType = 1 + iota
	OrderTypeIoc
	OrderTypePostOnly
	OrderTypeMarket
	OrderTypeUnknown
	OrderTypeFok
)

func (t OrderType) String() string {
	switch t {
	case OrderTypeLimit:
		return "Limit"
	case OrderTypeIoc:
		return "IOC"
	case OrderTypePostOnly:
		return "PostOnly"
	case OrderTypeMarket:
		return "Market"
	case OrderTypeFok:
		return "FOK"
	default:
		return "OrderTypeUnknown"
	}
}
func (s OrderType) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
func StrToOrderType(str string) OrderType {
	switch str {
	case "Limit":
		return OrderTypeLimit
	case "IOC":
		return OrderTypeIoc
	case "PostOnly":
		return OrderTypePostOnly
	case "Market":
		return OrderTypeMarket
	case "FOK":
		return OrderTypeFok
	default:
		return OrderTypeUnknown
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
// 下单信号
type SignalType int

const (
	SignalTypeNewOrder    SignalType = 1 + iota // 创建新订单
	SignalTypeCheckOrder                        // 检查订单
	SignalTypeCancelOrder                       // 取消订单
	SignalTypeGetPos                            // 获取仓位
	SignalTypeGetEquity                         // 获取资金
	SignalTypeAmend                             // 改单
	SignalTypeGetTicker
	SignalTypeGetLiteTicker
	SignalTypeGetIndex
	SignalTypeCancelOne
	SignalTypeCancelAll
)

func (t SignalType) String() string {
	switch t {
	case SignalTypeNewOrder:
		// return "创建订单"
		return "NewOrder"
	case SignalTypeCheckOrder:
		// return "检查订单"
		return "CheckOrder"
	case SignalTypeCancelOrder:
		// return "取消订单"
		return "CancelOrder"
	case SignalTypeGetPos:
		// return "获取仓位"
		return "GetPos"
	case SignalTypeGetEquity:
		// return "获取账户"
		return "GetEquity"
	case SignalTypeAmend:
		// return "修改订单"
		return "AmendOrder"
	case SignalTypeGetTicker:
		// return "获取Ticker"
		return "GetTicker"
	case SignalTypeGetLiteTicker:
		// return "获取LiteTicker"
		return "GetLiteTicker"
	case SignalTypeGetIndex:
		// return "获取Index"
		return "GetIndex"
	case SignalTypeCancelAll:
		return "CancelAll"
	case SignalTypeCancelOne:
		return "CancelOne"
	default:
		return "SignalType未知"
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
const (
	SignalChannelTypeWs = 1 + iota
	SignalChannelTypeRs
)

func ConvertSignalToLocalOrder(s *Signal) (o Order) {
	o.Amount = s.Amount
	o.EventTime = time.Now().UnixMilli()
	o.OrderType = s.OrderType
	o.ClientID = s.ClientID
	o.Price = s.Price
	o.OrderSide = s.OrderSide
	return
}

// easyjson:json
type Signal struct {
	SignalChannelType int         // 不指定的话，bq底层根据自我已知情况选择，一般是ws优先
	Time              int64       `json:"t,omitempty"`  // 信号产生时间 用于统计内部延迟 单位ns
	Type              SignalType  `json:"T,omitempty"`  // 指令类型
	ClientID          string      `json:"C,omitempty"`  // cid
	OrderID           string      `json:"O,omitempty"`  // oid
	Price             float64     `json:"p,omitempty"`  // 价格
	Amount            fixed.Fixed `json:"a,omitempty"`  // 数量
	OrderSide         OrderSide   `json:"os,omitempty"` // 方向  要特别注意 bq层下平仓单 不一定带 reduceOnly 可能会不带
	OrderType         OrderType   `json:"ot,omitempty"` // 下单类型
	ForceReduceOnly   bool        `json:"fr,omitempty"` // 强制平仓 bq层下平仓单 必须带 reduceOnly
	Pair              Pair
}

func (s *Signal) String() string {
	return fmt.Sprintf("[%s]pair:%s, cid:%s, oid:%s, px:%.10f, sz:%s, %s, %s",
		s.Type.String(),
		s.Pair.String(),
		s.ClientID,
		s.OrderID,
		s.Price,
		s.Amount.String(),
		s.OrderSide.String(),
		s.OrderType.String(),
	)
}

/*------------------------------------------------------------------------------------------------------------------*/
// 深度数据结构
// easyjson:json
type DepthItem struct {
	Price  float64 `json:"p"` // 原始价格
	Amount float64 `json:"a"` // 原始量
	// 注意 以下用于 lv2 => lv3 的统计量 一般用不上
	AddNum                int64   `json:"d,omitempty"` // 本档位量增加次数
	RdcNum                int64   `json:"r,omitempty"` // 本档位量减少次数
	BigAddAmt             float64 `json:"b,omitempty"` // 本档位量最大幅增加的量
	BigRdcAmt             float64 `json:"c,omitempty"` // 本档位量最大幅减少的量
	BigAddElapseTsMs      int64   `json:"w,omitempty"` // 本档位量最大幅增加的量发生的时间
	BigRdcElapseTsMs      int64   `json:"w,omitempty"` // 本档位量最大幅减少的量发生的时间
	AmtChgCumSum          float64 `json:"c,omitempty"` // 本档位量累积变化数量
	FirstAppearElapseTsMs int64   `json:"f,omitempty"` // 本档位第一次出现的时间戳
	LastUpdateElapseTsMs  int64   `json:"l,omitempty"` // 本档位最后一次更新的时间戳
	MinAddAmt             float64 `json:"t,omitempty"` // 本档位最小增加量
	MinRdcAmt             float64 `json:"m,omitempty"` // 本档位最小减少量
	TotalAddAmt           float64 `json:"a,omitempty"` // 本档位累积增加的量
	TotalRdcAmt           float64 `json:"r,omitempty"` // 本档位累积减少的量
}

type DepthItemAfterMerge struct {
	PriceVW   float64 `json:"p"`
	AmountSum float64 `json:"a"`
	// 注意 以下用于 lv2 => lv3 的统计量 一般用不上
	AddNumVW            float64 `json:"d,omitempty"`
	RdcNumVW            float64 `json:"r,omitempty"`
	AddNumSum           float64 `json:"d,omitempty"`
	RdcNumSum           float64 `json:"r,omitempty"`
	BigAddAmtVW         float64 `json:"b,omitempty"`
	BigAddElapseTsMsVW  float64 `json:"w,omitempty"`
	BigRdcAmtVW         float64 `json:"c,omitempty"`
	BigRdcElapseTsMsVW  float64 `json:"w,omitempty"`
	AmtChgVW            float64 `json:"c,omitempty"`
	FirstAppearElapseVW float64 `json:"f,omitempty"`
	LastUpdateElapseVW  float64 `json:"l,omitempty"`
	MinAddAmtVW         float64 `json:"t,omitempty"`
	MinRdcAmtVW         float64 `json:"m,omitempty"`
	TotalAddAmtVW       float64 `json:"a,omitempty"`
	TotalRdcAmtVW       float64 `json:"r,omitempty"`
}

type DepthMerge struct {
	Lock sync.RWMutex `json:"-"`
	Seq  Seq_T
	Bids []DepthItemAfterMerge `json:"b,omitemtpy"`
	Asks []DepthItemAfterMerge `json:"a,omitemtpy"`
}

// MidPrice returns the middle of Bid and Ask.
func (o *DepthMerge) Price() float64 {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	bp := 0.0
	if len(o.Bids) > 0 {
		bp = o.Bids[0].PriceVW
	}
	ap := 0.0
	if len(o.Asks) > 0 {
		ap = o.Asks[0].PriceVW
	}
	if bp == 0 || ap == 0 {
		return (bp + ap)
	}
	return (bp + ap) * 0.5
}

// easyjson:json
type Depth struct {
	Lock sync.RWMutex `json:"-"`
	Seq  Seq_T
	Bids []DepthItem `json:"b,omitemtpy"`
	Asks []DepthItem `json:"a,omitemtpy"`
}

// 获取ask bid 深度
func (o *Depth) GetDepth() (float64, float64, float64) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	var r, a, b float64
	lenAsks := len(o.Asks)
	lenBids := len(o.Bids)
	if lenAsks > 0 && lenBids > 0 {
		for _, v := range o.Asks {
			a += v.Amount
		}
		for _, v := range o.Bids {
			b += v.Amount
		}
		maxAsk := o.Asks[lenAsks-1].Price
		minBid := o.Bids[lenBids-1].Price
		r = (maxAsk - minBid) / (minBid + maxAsk)
	}
	return r, a, b
}

// GetDepthByDist 获取指定深度范围内的 ask bid 深度信息
func (o *Depth) GetDepthByDist(dist float64) (float64, float64, float64, float64) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	var bp, bq, ap, aq float64
	lenAsks := len(o.Asks)
	lenBids := len(o.Bids)
	if lenAsks > 0 && lenBids > 0 {
		ap0 := o.Asks[0].Price
		bp0 := o.Bids[0].Price
		for _, v := range o.Asks {
			if math.Abs(v.Price-ap0)/ap0 > dist {
				break
			}
			aq += v.Amount
			ap = v.Price
		}
		for _, v := range o.Bids {
			if math.Abs(v.Price-bp0)/bp0 > dist {
				break
			}
			bq += v.Amount
			bp = v.Price
		}
	}
	return bp, bq, ap, aq
}

// Ask 卖一
func (o *Depth) Ask() (result DepthItem) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	if len(o.Asks) > 0 {
		result = o.Asks[0]
	}
	return
}

// Bid 买一
func (o *Depth) Bid() (result DepthItem) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	if len(o.Bids) > 0 {
		result = o.Bids[0]
	}
	return
}

// BidPrice 买一价
func (o *Depth) BidPrice() (result float64) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	if len(o.Bids) > 0 {
		result = o.Bids[0].Price
	}
	return
}

// AskPrice 卖一价
func (o *Depth) AskPrice() (result float64) {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	if len(o.Asks) > 0 {
		result = o.Asks[0].Price
	}
	return
}

// MidPrice returns the middle of Bid and Ask.
func (o *Depth) Price() float64 {
	o.Lock.RLock()
	defer o.Lock.RUnlock()
	bp := 0.0
	if len(o.Bids) > 0 {
		bp = o.Bids[0].Price
	}
	ap := 0.0
	if len(o.Asks) > 0 {
		ap = o.Asks[0].Price
	}
	if bp == 0 || ap == 0 {
		return (bp + ap)
	}
	return (bp + ap) * 0.5
}
func (d *Depth) CopyContent() (d0 Depth) {
	d.Lock.RLock()
	d0.Asks = make([]DepthItem, 0, len(d.Asks))
	d0.Bids = make([]DepthItem, 0, len(d.Bids))
	d0.Asks = append(d0.Asks, d.Asks...)
	d0.Bids = append(d0.Bids, d.Bids...)
	d0.Seq = d.Seq
	d.Lock.RUnlock()
	return
}

func (d *Depth) String() string {
	d.Lock.RLock()
	defer d.Lock.RUnlock()
	strBuf := bytes.Buffer{}
	strBuf.WriteString("bids:")
	for _, v := range d.Bids {
		strBuf.WriteString(strconv.FormatFloat(v.Price, 'f', -1, 64))
		strBuf.WriteString(",")
		strBuf.WriteString(strconv.FormatFloat(v.Amount, 'f', -1, 64))
		strBuf.WriteString(";")
	}
	strBuf.WriteString("asks:")
	for _, v := range d.Asks {
		strBuf.WriteString(strconv.FormatFloat(v.Price, 'f', -1, 64))
		strBuf.WriteString(",")
		strBuf.WriteString(strconv.FormatFloat(v.Amount, 'f', -1, 64))
		strBuf.WriteString(";")
	}
	return strBuf.String()
}

/*------------------------------------------------------------------------------------------------------------------*/
// 成交方向
type TradeSide int

const (
	TradeSideBuy TradeSide = 1 + iota
	TradeSideSell
)

/*------------------------------------------------------------------------------------------------------------------*/

// Trade 成交数据结构 记录taker方向
type Trade struct {
	Lock    sync.RWMutex `json:"-"`
	Pair    Pair         `json:"p,omitempty"`
	MinFill float64      `json:"m,omitempty"`
	MaxFill float64      `json:"M,omitempty"`
	BuyNum  float64      `json:"b,omitempty"`
	SellNum float64      `json:"s,omitempty"`
	BuyQ    float64      `json:"B,omitempty"`
	SellQ   float64      `json:"S,omitempty"`
	BuyV    float64      `json:"bv,omitempty"`
	SellV   float64      `json:"sv,omitempty"`
	IdEx    string       `json:"id,omitempty"`
	TsMsEx  int64        `json:"ts,omitempty"`
}

// Update 更新成交信息  每次收到ws成交推送请调用本方法
// 一般用在实盘环境中 ws 在 2tick 之间收到 trade 推送 累计增加成交信息
// side is taker trade side
func (t *Trade) Update(side TradeSide, amount float64, price float64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	switch side {
	case TradeSideBuy:
		t.BuyNum += 1
		t.BuyQ += amount
		t.BuyV += amount * price
	case TradeSideSell:
		t.SellNum += 1
		t.SellQ += amount
		t.SellV += amount * price
	}
	if t.MinFill == 0 {
		t.MinFill = price
	}
	if t.MaxFill == 0 {
		t.MaxFill = price
	}
	if price > t.MaxFill {
		t.MaxFill = price
	}
	if price < t.MinFill {
		t.MinFill = price
	}
}
func (t *Trade) Update2(side TradeSide, amount float64, price float64, IdEx string, TsMsEx int64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	switch side {
	case TradeSideBuy:
		t.BuyNum += 1
		t.BuyQ += amount
		t.BuyV += amount * price
	case TradeSideSell:
		t.SellNum += 1
		t.SellQ += amount
		t.SellV += amount * price
	}
	if t.MinFill == 0 {
		t.MinFill = price
	}
	if t.MaxFill == 0 {
		t.MaxFill = price
	}
	if price > t.MaxFill {
		t.MaxFill = price
	}
	if price < t.MinFill {
		t.MinFill = price
	}
	t.IdEx = IdEx
	t.TsMsEx = TsMsEx
}

// Get 获取成交信息 并重置 策略层面需要获取成交信息汇总备用的时候 请调用本方法 会重置成交信息从头累计
// 一般用于实盘环境中 但是要确保每个 tick 有且仅有 1 次调用
func (t *Trade) Get() (float64, float64, float64, float64, float64, float64, float64, float64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	// get
	maxFill := t.MaxFill
	minFill := t.MinFill
	buyNum := t.BuyNum
	sellNum := t.SellNum
	buyQ := t.BuyQ
	sellQ := t.SellQ
	buyV := t.BuyV
	sellV := t.SellV
	// reset
	t.MinFill = 0.0
	t.MaxFill = 0.0
	t.BuyNum = 0.0
	t.SellNum = 0.0
	t.BuyQ = 0.0
	t.SellQ = 0.0
	t.BuyV = 0.0
	t.SellV = 0.0
	return maxFill, minFill, buyNum, sellNum, buyQ, sellQ, buyV, sellV
}
func (t *Trade) Get2() (float64, float64, float64, float64, float64, float64, float64, float64, string, int64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	// get
	maxFill := t.MaxFill
	minFill := t.MinFill
	buyNum := t.BuyNum
	sellNum := t.SellNum
	buyQ := t.BuyQ
	sellQ := t.SellQ
	buyV := t.BuyV
	sellV := t.SellV
	// reset
	t.MinFill = 0.0
	t.MaxFill = 0.0
	t.BuyNum = 0.0
	t.SellNum = 0.0
	t.BuyQ = 0.0
	t.SellQ = 0.0
	t.BuyV = 0.0
	t.SellV = 0.0
	return maxFill, minFill, buyNum, sellNum, buyQ, sellQ, buyV, sellV, t.IdEx, t.TsMsEx
}

// Read 获取成交信息 不重置
// 一般用在回测中 获取成交信息的时候 多处调用 不会重置本tick成交信息
func (t *Trade) Read() (float64, float64, float64, float64, float64, float64, float64, float64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	// get
	maxFill := t.MaxFill
	minFill := t.MinFill
	buyNum := t.BuyNum
	sellNum := t.SellNum
	buyQ := t.BuyQ
	sellQ := t.SellQ
	buyV := t.BuyV
	sellV := t.SellV
	return maxFill, minFill, buyNum, sellNum, buyQ, sellQ, buyV, sellV
}

// Load 加载成交信息 不重置 策略层面需要获取成交信息汇总备用的时候 请调用本方法 不会从头累计
// 一般用在回测中 加载历史成交信息后 放入在 trade 结构体中
func (t *Trade) Load(maxFill float64, minFill float64, buyNum float64, sellNum float64, buyQ float64, sellQ float64, buyV float64, sellV float64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	t.MaxFill = maxFill
	t.MinFill = minFill
	t.BuyNum = buyNum
	t.SellNum = sellNum
	t.BuyQ = buyQ
	t.SellQ = sellQ
	t.BuyV = buyV
	t.SellV = sellV
}

// Add 加载成交信息 只要不被取用一直累积
// 一般用在回测中 加载历史成交信息后 放入在 trade 结构体中
func (t *Trade) Add(maxFill float64, minFill float64, buyNum float64, sellNum float64, buyQ float64, sellQ float64, buyV float64, sellV float64) {
	t.Lock.Lock()
	defer t.Lock.Unlock()
	if maxFill > t.MaxFill && maxFill > 0 {
		t.MaxFill = maxFill
	}
	if (minFill < t.MinFill && minFill > 0) || (t.MinFill == 0 && minFill > 0) {
		t.MinFill = minFill
	}
	t.BuyNum += buyNum
	t.SellNum += sellNum
	t.BuyQ += buyQ
	t.SellQ += sellQ
	t.BuyV += buyV
	t.SellV += sellV
}

/*------------------------------------------------------------------------------------------------------------------*/
type Kline struct {
	// Lock        sync.RWMutex
	Symbol      string
	OpenTimeMs  int64
	CloseTimeMs int64
	EventTimeMs int64
	LocalTimeMs int64
	//kline variables
	Open         float64
	Close        float64
	High         float64
	Low          float64
	BuyNotional  float64 //quote
	BuyVolume    float64 //base
	SellNotional float64 //quote
	SellVolume   float64 //base
	BuyTradeNum  int64
	SellTradeNum int64
}

func (t *Kline) String() string {
	// t.Lock.Lock()
	// defer t.Lock.Unlock()
	return fmt.Sprintf("Symbol: %s OpenTimeMs:%d CloseTimeMs:%d EventTimeMs:%d LocalTimeMs:%d Open:%f Close:%f High:%f Low:%f BuyNotional:%f BuyVolume:%f SellNotional:%f SellVolume:%f BuyTradeNum:%d SellTradeNum:%d",
		t.Symbol, t.OpenTimeMs, t.CloseTimeMs, t.EventTimeMs, t.LocalTimeMs, t.Open, t.Close, t.High, t.Low, t.BuyNotional, t.BuyVolume, t.SellNotional, t.SellVolume, t.BuyTradeNum, t.SellTradeNum)
}

func NewKline(symbol string, OpenTimeMs, CloseTimeMs, EventTimeMs int64, open, close, high, low float64, buyNum, sellNum, buyQ, sellQ, buyV, sellV float64, tsNs int64) Kline {
	var t Kline
	t.Symbol = symbol
	t.OpenTimeMs = OpenTimeMs
	t.CloseTimeMs = CloseTimeMs
	t.EventTimeMs = EventTimeMs
	t.LocalTimeMs = tsNs / 1e6

	t.BuyNotional = buyV
	t.BuyVolume = buyQ
	t.SellNotional = sellV
	t.SellVolume = sellQ
	t.BuyTradeNum = int64(buyNum)
	t.SellTradeNum = int64(sellNum)

	t.Open = open
	t.Close = close
	t.High = high
	t.Low = low
	return t
}

/*------------------------------------------------------------------------------------------------------------------*/

// 交易信息集合
type TradeMsg struct {
	Name BrokerName // 交易所名称
	Pair Pair       // 交易对
	// Position Pos         // 账户的仓位
	// Equity   Equity      // 账户资产
	// RsEquity Equity      // rs GetEquity获取的资金余额，部分交易所的ws返回不满足策略层使用，要用rs区别
	Ticker Ticker // bbo 1档orderbook信息 更快 信息更少
	// Index    Index       // 指数信息
	// 要特别注意 !!!
	// 严禁把来自 snapshot 的 orderbook 信息 放入 DepthInc
	// 严禁把来自增量信息的 orderbook 信息 放入 Depth
	Depth      Depth                                 // n档orderbook信息 来自 snapshot 信息
	DepthInc   Depth                                 // n档orderbook信息 从增量信息出来
	DepthMerge DepthMerge                            // n档orderbook信息 从增量信息出来后 进行聚合得到
	Orderbook  *orderbook_mediator.OrderbookMediator // 订单簿信息
	Trade      Trade                                 // 成交信息
	// 以下为 ALPHA Research 需要的信息 普通策略无需关注
	HasEvent atomic.Bool    // tick间是否有事件发生
	RpMid    atomic.Float64 // rp mid price
	IsValid  atomic.Bool    // 当前盘口是否在开盘中
}

type AcctMsg struct {
	Name     BrokerName // 交易所名称
	Pair     Pair       // 交易对
	Position Pos        // 账户的仓位
	Equity   Equity     // 账户资产
}

/*------------------------------------------------------------------------------------------------------------------*/

// 回调函数集合 broker接口需要用到的所有回调函数放这里
// 统一规范 cbfunc中不允许有高耗时操作 如果有则必须在cb func内起go程异步执行
type CallbackFunc struct {
	/* 延迟敏感 */
	OnTicker                 func(ts int64) // 不需要传递参数 更新在trademsg
	OnIndex                  func(ts int64, event IndexEvent)
	OnMark                   func(ts int64, event MarkEvent)
	OnDepth                  func(ts int64) // 不需要传递参数 更新在trademsg
	OnPositionEvent          func(ts int64, event PositionEvent)
	OnEquityEvent            func(ts int64, event EquityEvent)
	OnMarketLiquidationEvent func(ts int64, event MarketLiquidationEvent)
	OnTradeEvent             func(ts int64, event TradeEvent)
	OnTrade                  func(ts int64)                   // 不需要传递参数 更新在trademsg
	OnKline                  func(ts int64, kline Kline)      // 不需要传递参数 更新在trademsg
	OnOrder                  func(ts int64, event OrderEvent) // 需要传递参数 订单事件
	/* 延迟不敏感 */
	OnExit           func(msg string)    // 需要传递参数 停机原因
	OnReset          func(msg string)    // 需要传递参数 重置原因
	OnMsg            func(msg string)    // 需要传递参数 把底层信息传递到策略层 策略层可以发送丁丁也可以做其他操作
	OnDetail         func(msg string)    // 抛出http请求过程细节信息, 当onDetail 为nil 表示应用层不关心底层细节 直接bypass掉
	OnWsReady        func(ex BrokerName) // 交易所ws ready时(所有订阅成功)通知上层
	OnWsReconnecting func(ex BrokerName) // 交易所ws 重连时通知上层, 订阅成功后会发出OnWsReady(只实现了部分交易所2024-9-30)
	OnWsReconnected  func(ex BrokerName) // 交易所ws 重连成功后通知上层
	OnExchangeDown   func()              // 交易所ws 重连成功后通知上层

	// utf使用
	Collect func(tsns int64, msg string, dataType any)
}

/*------------------------------------------------------------------------------------------------------------------*/
// 使用环形队列实现 rollingMa
type RingMa struct {
	queue []float64
	size  int
	fsize float64
	index int
	first bool
	sum   float64
	Avg   float64
}

func NewRingMa(size int) *RingMa {
	if size <= 0 {
		panic("size must be greater than 0")
	}
	return &RingMa{
		queue: make([]float64, size),
		size:  size,
		fsize: float64(size),
		first: true,
		index: -1,
	}
}

func (r *RingMa) GetQueue() []float64 {
	return r.queue
}

func (r *RingMa) GetSize() int {
	return r.size
}

func (r *RingMa) GetSum() float64 {
	return r.sum
}

func (r *RingMa) ResetToSize(size int, input float64) {
	if size <= 0 {
		return
	}
	newqueue := make([]float64, size)

	newsum := 0.0
	for i := 0; i < size; i++ {
		newqueue[i] = input
		newsum += input
	}
	r.sum = newsum
	r.fsize = float64(size)
	r.queue = newqueue
	r.size = size
	r.first = false
	r.index = size - 1
	r.Avg = r.sum / r.fsize
}

func (r *RingMa) Update(n float64) float64 {
	r.index++
	if r.index == r.size {
		r.index = 0
		if r.first {
			r.first = false
		}
	}
	r.sum += n
	r.sum -= r.queue[r.index]
	r.queue[r.index] = n
	if r.first {
		r.Avg = r.sum / float64(r.index+1)
	} else {
		r.Avg = r.sum / r.fsize
	}
	return r.Avg
}

// SetSize 改变平滑window的最大长度
func (r *RingMa) SetSize(size int) {
	if size <= 0 {
		return
	}
	// 输入Size小于等于原有size，不改变
	if size == r.size {
		return
	}
	// 当新size大于原有size时
	if size > r.size {
		newQueue := make([]float64, size)
		if r.first {
			// 如果是第一遍赋值，直接把队列中原有的值整体搬到新的队列中
			copy(newQueue, r.queue[:r.index+1])
		} else {
			// 如果已经是第二遍赋值，把队列中原有的值按照从旧到新的顺序排列到新的队列中
			copy(newQueue[:r.size-r.index-1], r.queue[r.index+1:])
			copy(newQueue[r.size-r.index-1:], r.queue[:r.index+1])
			r.index = r.size - 1
		}
		// 更新各项标志值
		r.queue = newQueue
		r.size = size
		r.fsize = float64(size)
		r.first = true
	} else { // 当新size值小于原有的size时
		newQueue := make([]float64, size)
		if r.index+1 >= size {
			// 当新size比原有的index标志前面的长度还小
			copy(newQueue, r.queue[r.index+1-size:r.index+1])
			r.index = size - 1
			r.first = true
			r.sum = 0.0
			r.queue = newQueue
			r.size = size
			r.fsize = float64(size)
			for _, val := range r.queue {
				r.sum += val
			}
			r.Avg = r.sum / r.fsize
		} else {
			// 当新size比原有的index标志前面的长度要长
			if r.first { // 再且当原有队列是第一次赋值时，直接把原有队列整体搬到新队列即可
				copy(newQueue, r.queue[:r.index+1])
				r.queue = newQueue
				r.size = size
				r.fsize = float64(size)
			} else { // 再且当原有队列不是第一次赋值时，把原有队列队尾部分 以及 把index标志前面的长度按顺序放进去新队列中
				copy(newQueue[size-r.index-1:], r.queue[:r.index+1])
				copy(newQueue[:size-r.index-1], r.queue[r.size-size+r.index+1:])
				r.index = size - 1
				r.first = true
				r.sum = 0.0
				r.queue = newQueue
				r.size = size
				r.fsize = float64(size)
				for _, val := range r.queue {
					r.sum += val
				}
				r.Avg = r.sum / r.fsize
			}
		}
	}
}

// RollingMa 计算滚动平均数的结构体
// 会存在memcpy的问题 性能比较弱 但是可以动态改变window
type RollingMa struct {
	lock sync.Mutex
	sum  float64
	list []float64
	avg  float64
	N    int
}

func NewRollingMa(n int) *RollingMa {
	r := &RollingMa{
		N: n,
	}
	return r
}
func (r *RollingMa) GetSum() float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.sum
}

func (r *RollingMa) GetList() []float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.list
}

// SetN 改变平滑window
func (r *RollingMa) SetN(n int) {
	if n > 0 {
		r.lock.Lock()
		defer r.lock.Unlock()
		r.N = n
	}
}

// Update 输入最新值并返回当前ma
func (r *RollingMa) Update(n float64) float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	// step 1
	r.sum += n
	r.list = append(r.list, n)
	// step 2
	if len(r.list) > r.N {
		r.sum -= r.list[0]
		r.list = r.list[1:]
	}
	// step 3
	ma := r.sum / float64(len(r.list))
	return ma
}

// RollingDes 计算滚动的最大最小值
type RollingMaxMinV1 struct {
	lock sync.Mutex
	sum  float64
	list []float64
	avg  float64
	N    int
}

type RollingMaxMin struct {
	lock sync.Mutex
	list []float64
	sum  float64
	N    int
	// 用于维护滑动窗口最大、最小值的双端队列，存储索引
	dqMax []int64
	dqMin []int64
}

func NewRollingMaxMinV1(n int) *RollingMaxMinV1 {
	r := &RollingMaxMinV1{
		N:    n,
		list: make([]float64, 0, n),
	}
	return r
}

func NewRollingMaxMin(n int) *RollingMaxMin {
	if n <= 0 {
		panic("size must be greater than 0")
	}
	return &RollingMaxMin{
		N:     n,
		list:  make([]float64, 0, n),
		dqMax: make([]int64, 0, n),
		dqMin: make([]int64, 0, n),
	}
}

// Update 输入最新值并返回当前ma
func (r *RollingMaxMinV1) Update(n float64) (float64, float64, float64) {
	r.lock.Lock()
	defer r.lock.Unlock()
	// step 1
	r.sum += n
	r.list = append(r.list, n)
	// step 2
	if len(r.list) > r.N {
		r.sum -= r.list[0]
		r.list = r.list[1:]
	}
	// step 3
	ma := r.sum / float64(len(r.list))
	min_ := 999999999.0
	max_ := -999999999.0
	for _, v := range r.list {
		if v > max_ {
			max_ = v
		}
		if v < min_ {
			min_ = v
		}
	}
	return ma, max_, min_
}

// Update V2
func (r *RollingMaxMin) Update(n float64) (float64, float64, float64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// 维护总和
	r.sum += n
	r.list = append(r.list, n)

	// 弹出dqMax尾部小于新值的索引
	for len(r.dqMax) > 0 && r.list[r.dqMax[len(r.dqMax)-1]] < n {
		r.dqMax = r.dqMax[:len(r.dqMax)-1]
	}
	r.dqMax = append(r.dqMax, int64(len(r.list)-1))

	// 弹出dqMin尾部大于新值的索引
	for len(r.dqMin) > 0 && r.list[r.dqMin[len(r.dqMin)-1]] > n {
		r.dqMin = r.dqMin[:len(r.dqMin)-1]
	}
	r.dqMin = append(r.dqMin, int64(len(r.list)-1))

	// 如果窗口超出大小，移除最早元素
	if len(r.list) > r.N {
		oldVal := r.list[0]
		r.sum -= oldVal
		r.list = r.list[1:]

		// 更新队列中索引
		for i := 0; i < len(r.dqMax); i++ {
			r.dqMax[i] = r.dqMax[i] - 1
		}
		for i := 0; i < len(r.dqMin); i++ {
			r.dqMin[i] = r.dqMin[i] - 1
		}

		// 如果队列头部已移除，则弹出
		if len(r.dqMax) > 0 && r.dqMax[0] < 0 {
			r.dqMax = r.dqMax[1:]
		}
		if len(r.dqMin) > 0 && r.dqMin[0] < 0 {
			r.dqMin = r.dqMin[1:]
		}
	}

	ma := r.sum / float64(len(r.list))
	maxVal := r.list[r.dqMax[0]]
	minVal := r.list[r.dqMin[0]]
	return ma, maxVal, minVal
}

func (r *RollingMaxMin) GetMinMaxIndex() (int64, int64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// 若双端队列为空，直接返回
	if len(r.dqMax) == 0 || len(r.dqMin) == 0 {
		return -1, -1
	}

	// 取队首位置对应的全局索引
	maxIdx := r.dqMax[0]
	minIdx := r.dqMin[0]
	return maxIdx, minIdx
}

func (r *RollingMaxMin) GetList() []float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.list
}

func (r *RollingMaxMin) GetFirst() float64 {
	r.lock.Lock()
	defer r.lock.Unlock()

	// 若双端队列为空，直接返回
	if len(r.list) > 0 {
		return r.list[0]
	} else {
		return 0
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
type RollingStd struct {
	data         []float64
	windowSize   int
	sum          float64
	sumOfSquares float64
	index        int
	count        int
}

func NewRollingStd(windowSize int) *RollingStd {
	return &RollingStd{
		data:       make([]float64, windowSize),
		windowSize: windowSize,
	}
}

func (r *RollingStd) Update(value float64) float64 {
	if r.count < r.windowSize {
		// Initial filling of the window
		r.data[r.index] = value
		r.sum += value
		r.sumOfSquares += value * value
		r.count++
	} else {
		// Sliding window update
		oldValue := r.data[r.index]
		r.data[r.index] = value

		r.sum += value - oldValue
		r.sumOfSquares += value*value - oldValue*oldValue
	}

	r.index = (r.index + 1) % r.windowSize

	if r.count < r.windowSize {
		return 0.0
	}

	mean := r.sum / float64(r.windowSize)
	variance := (r.sumOfSquares / float64(r.windowSize)) - (mean * mean)
	return math.Sqrt(variance)
}

/*------------------------------------------------------------------------------------------------------------------*/

// AcctSum 账户概览 包含 持币和持仓信息 主要用于账户监控
type AcctSum struct {
	Lock           sync.RWMutex    // 读写锁
	Balances       []BalanceSum    // 存放 持币数量 现货合约均有持币信息 币名字全部用小写
	Positions      []PositionSum   // 存放 net pos 净持仓量 仅合约存在持仓信息
	BalancesDetail []BalanceDetail // 提供详尽信息
}

func (s *AcctSum) String() string {
	s.Lock.RLock()
	defer s.Lock.RUnlock()
	var msg string
	for _, v := range s.Balances {
		msg += fmt.Sprintf("[持币][%v] 数量:%v 可用:%v 币价:%v Notional:%v\n", v.Name, v.Amount, v.Avail, v.Price, v.Avail*v.Price)
	}
	for _, v := range s.Positions {
		msg += fmt.Sprintf("[仓位][%v] 数量:%v 均价:%v \n", v.Name, v.Amount, v.Ave)
	}
	return msg
}

func NewAcctSum() AcctSum {
	var a AcctSum
	a.Balances = make([]BalanceSum, 0)
	a.Positions = make([]PositionSum, 0)
	return a
}

type BalanceSum struct {
	Name   string  // 币种名字 全部用小写
	Price  float64 // 币种价格 计价币本身可能忘记设置。0就是1，应用层做好判断
	Amount float64 // 币种数量
	Avail  float64 // 可用币种数量 有可能被当成保证金占用了
}

func (b *BalanceSum) String() string {
	return fmt.Sprintf("[持币][%v] 数量:%v 可用:%v 币价:%v Notional:%v\n", b.Name, b.Amount, b.Avail, b.Price, b.Avail*b.Price)
}

const (
	BalanceSumNoteMap_TotalNotIncludeUpl   = 1 << iota // 总额不包含浮盈浮亏
	BalanceSumNoteMap_TotalNotIncludeAvail = 1 << iota // 总额不包含可用余额
	BalanceSumNoteMap_UplIsInvalid         = 1 << iota // 浮盈浮亏字段无效，不应使用
)

// 提供详尽信息的余额数据
type BalanceDetail struct {
	Name    string  // 币种名字 全部用小写
	Total   float64 // 币种总额。包含可用余额；是否包含浮盈浮亏，要看NoteMap的TotalNotIncludeUpl设置
	Upl     float64 // 浮盈浮亏，0不一定表示没有浮盈，要看NoteMap的UplIsInvalid
	Avail   float64 // 可用余额
	NoteMap int64   // 字段说明，默认0表示: Total包含可用余额、浮盈浮亏；Upl字段有效；Avail字段有效
}

type PosSide int

func (s PosSide) String() string {
	switch s {
	case PosSideLong:
		return "long"
	case PosSideShort:
		return "short"
	}
	return "unknown"
}

const (
	PosSideLong PosSide = iota + 1
	PosSideShort
)

type PosMode int

const (
	PosModeNil    PosMode = iota // 单向持仓
	PosModeOneway                // 单向持仓
	PosModeHedge
)

func (s PosMode) String() string {
	switch s {
	case PosModeOneway:
		return "oneway"
	case PosModeHedge:
		return "hedge"
	}
	return "unknown"
}

// 2023-12-3添加了Side和Mode，方便统一清仓模板，Amount可以用非负数表示，但有历史代码没改，可能会存在负数。
type PositionSum struct {
	Name        string  // 交易对名字, pair.String(), like btc_usdt
	Amount      float64 // 持仓大小 .
	AvailAmount float64 // 可用持仓
	Ave         float64 // 平均持仓价格
	Pnl         float64 // 根据当前mp价格和仓位ave价格计算的浮盈浮亏
	PositionId  string
	Side        PosSide
	Mode        PosMode
	MarginMode  MarginMode // 保证金模式，大多数所不需要，bg v2需要
	Seq         Seq_T
}

func (p *PositionSum) String() string {
	return fmt.Sprintf("PositionSum[Name:%s,Amount:%v,Ave:%f,AvailAmount:%v,Side:%d,Mode:%d,MarginMode:%s]",
		p.Name, p.Amount, p.Ave, p.AvailAmount, p.Side, p.Mode, p.MarginMode.String())
}

type FundingRate struct {
	Pair              Pair
	FundingTimeMS     int64   // 当期 结算时间, 毫秒时间戳。有些交易所例如kucoin返回的是剩余时间数，要取当前时间戳运算，所以不一定完全精确，可能会有几毫秒差别。
	Rate              float64 // > 0 表示多头付空头资金费
	NextFundingTimeMS int64   // 下一期
	NextRate          float64 // 下一次的费率, 只有 NextFundingTimeMS > 0 时才有效
	UpdateTimeMS      int64   // 更新时间, 毫秒时间戳
	IntervalHours     int     // 结算周期（有些交易所没有）
	// RateFixed     bool // 结算周期内费率是否固定不变
}

func (fr *FundingRate) String() string {
	return fmt.Sprintf("FundingRate: Pair:%v FundingTimeMS:%v Rate:%v NextFundingTimeMS:%v NextRate:%v UpdateTimeMS:%v IntervalHours:%d", //
		fr.Pair.String(), fr.FundingTimeMS, fr.Rate, fr.NextFundingTimeMS, fr.NextRate, fr.UpdateTimeMS, fr.IntervalHours)
}

type ExState int

const (
	ExStateUnknow     = iota // 运行中，调用 BeforeTrade成功后进入的状态
	ExStateRunning           // 运行中，调用 BeforeTrade成功后进入的状态
	ExStateAfterTrade        // 暂停中，调用 AfterTrade 后进入的状态. AfterTrade不释放资源
)

type SymbolMode int

const (
	SymbolMode_One   SymbolMode = iota
	SymbolMode_Multi            // 指定 pairs
	SymbolMode_All              // 不指定
)

// 保证金模式
type MarginMode int

const (
	MarginMode_Nil   MarginMode = iota
	MarginMode_Cross            //  全仓
	MarginMode_Iso              // 逐仓
)

func (mm MarginMode) String() string {
	switch mm {
	case MarginMode_Cross:
		return "c"
	case MarginMode_Iso:
		return "i"
	}
	return ""
}

type PriceLimit struct {
	BuyLimit  float64
	SellLimit float64
}

// 注意事项
type Caution struct {
	AddedDate string
	Msg       string
}

// 费率。适应partial marker/taker，长时间段可信，短时间段可能不准
type DualFeeRate struct {
	High float64
	Low  float64
}

func NewDualFee() DualFeeRate {
	return DualFeeRate{High: -1, Low: 1}
}

func (f *DualFeeRate) Update(feeRate float64) {
	if feeRate > f.High {
		f.High = feeRate
	}
	if feeRate < f.Low {
		f.Low = feeRate
	}
}

/*------------------------------------------------------------------------------------------------------------------*/
// 统一所有策略的版本信息输出格式

type AppVersionInfo struct {
	ApplicationName string `json:"ApplicationName"` // 策略名称 例如 beastStraDino
	BuildTime       string `json:"Build Time"`      // 编译时间 例如 2023-08-17T12:54:06
	AppVersion      string `json:"AppVersion"`      // 应用版本号，系统负责人自定义格式
	GitCommitHash   string `json:"GitCommitHash"`   // git commit hash 例如 531deda4eb3eff793a0f64ba6655dbebc785121e
	GitTag          string `json:"GitTag"`          // 		例如 v1.0.0
	GoVersion       string `json:"GoVersion"`       // 例如 go1.16.5
	QuantCommitHash string `json:"QuantCommitHash"` // 例如 e3423f3163cbae5fe624b420c6340ba2c98d9702
}

var VersionInfoSpliter = "\n@@@VersionInfoOnly@@@\n"

func BuildVersionInfo(info AppVersionInfo) string {
	jsonData, err := json.Marshal(info)
	if err != nil {
		return ""
	} else {
		return VersionInfoSpliter + string(jsonData) + VersionInfoSpliter
	}
}

func ParseVersionInfo(msg string) AppVersionInfo {
	var a AppVersionInfo
	msg = strings.TrimSuffix(strings.TrimPrefix(msg, VersionInfoSpliter), VersionInfoSpliter)
	json.Unmarshal([]byte(msg), &a)
	return a
}

func IsQuoteCoin(coin string) bool {
	for _, c := range []string{"usdc", "usdt", "fdusd", "busd", "ust"} {
		if strings.EqualFold(coin, c) {
			return true
		}
	}
	return false
}

/*------------------------------------------------------------------------------------------------------------------*/
// 币保证金账户信息
type UMAcctInfo struct {
	TsS                    int64
	MaintenanceMarginRate  float64
	TotalMaintenanceMargin float64
	TotalEquity            float64
	DiscountedTotalEquity  float64
	AccountStatus          string
}

func (u *UMAcctInfo) String() string {
	return fmt.Sprintf("UMAcctInfo[TsS:%v MaintenanceMarginRate:%v TotalMaintenanceMargin:%v TotalEquity:%v DiscountedTotalEquity:%v AccountStatus:%v]",
		u.TsS, u.MaintenanceMarginRate, u.TotalMaintenanceMargin, u.TotalEquity, u.DiscountedTotalEquity, u.AccountStatus)
}

type Positionhistory struct {
	Symbol string
	Pair   string
	//
	OpenTime      int64   // 开仓时间
	CloseTime     int64   // 平仓时间
	OpenPrice     float64 // 开仓价格
	CloseAvePrice float64 // 平仓均价
	OpenedAmount  float64 // 开了的仓位
	ClosedAmount  float64 // 已平仓量
	PnlAfterFees  float64 // 平仓盈亏
	Fee           float64 // 手续费
}

func (p *Positionhistory) String() string {
	return fmt.Sprintf("Positionhistory[Symbol:%v Pair:%v OpenTime:%v CloseTime:%v OpenPrice:%v CloseAvePrice:%v OpenedAmount:%v ClosedAmount:%v PnlAfterFees:%v Fee:%v]",
		p.Symbol, p.Pair, p.OpenTime, p.CloseTime, p.OpenPrice, p.CloseAvePrice, p.OpenedAmount, p.ClosedAmount, p.PnlAfterFees, p.Fee)
}

type PosHistResponse struct {
	Pos     []Positionhistory // 根据订单创建时间，从旧到新排序
	HasMore bool              // 表示在给定的时间范围内，是否还有订单没返回。如果是true，调用者可以根据Data最后一个订单的时间发起新请求
}

type CidWithPair struct {
	Cid  string
	Pair Pair
}

type CidsWithPair struct {
	Cids *[]string
	Pair Pair
}
type MarketLiquidationEvent struct {
	Symbol         string
	OrderType      OrderType
	Side           OrderSide
	OrderQuantity  fixed.Fixed
	OrderPrice     fixed.Fixed
	FilledPrice    float64
	FilledQuantity float64
	EventTimeMs    int64
	TradeTimeMs    int64
}

func (e *MarketLiquidationEvent) String() string {
	return fmt.Sprintf("MarketLiquidationEvent[Symbol:%v OrderType:%v Side:%v OrderQuantity:%v OrderPrice:%v FilledPrice:%v FilledQuantity:%v EventTimeMs:%v TradeTimeMs:%v]",
		e.Symbol, e.OrderType, e.Side, e.OrderQuantity, e.OrderPrice, e.FilledPrice, e.FilledQuantity, e.EventTimeMs, e.TradeTimeMs)
}

type AcctConfig struct {
	MarginMode  MarginMode
	PosMode     PosMode
	MaxLeverage int
}

func (a *AcctConfig) IsEmpty() bool {
	return a.MarginMode == MarginMode_Nil && a.PosMode == PosModeNil && a.MaxLeverage == 0
}

// 网页版本浏览器
var UserAgentSlice = []string{
	// 一、Chrome浏览器
	// Windows版：
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.361",
	// macOS版：
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.361",
	// Linux版：
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.361",
	// 二、Firefox浏览器
	// Windows版：
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:75.0) Gecko/20100101 Firefox/75.03",
	// macOS版：
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:75.0) Gecko/20100101 Firefox/75.03",
	// 三、Safari浏览器
	// macOS版：
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.5 Safari/605.1.153",
}
