package base

const (
	FeatGetFee = iota
)

type RsFeatures interface {
	GetFeatures() Features
}

// 我们已经接入的交易所功能
type Features struct {
	ExchangeInfo_1000XX       bool `n:"去除交易对'1000'字符信息"`
	ExchangeInfo_SetRiskLimit bool `n:"exchange info已设置最大持仓限额"`
	GetFee                    bool `n:"从交易所获取正确费率"`
	GetOrderList              bool `n:"获取订单列表\n由于手工交易"`
	GetFundingRate            bool `n:"获取资金费"`
	GetIndex                  bool `n:"获取Index Price"`
	OrderIOC                  bool `n:"下ioc订单"`
	OrderPostonly             bool `n:"下post only订单"`
	UpdatePosWithSeq          bool `n:"seq机制更新pos,保障旧数据不覆盖"`
	UpdateWsTickerWithSeq     bool `n:"seq机制更新ws ticker, 保障旧数据不覆盖"`
	UpdateWsDepthWithSeq      bool `n:"seq机制更新ws depth"`
	MultiSymbolOneAcct        bool `n:"一个账户可以多币对交易"` // 下单，但不一定支持查单撤单 2024-7-22
	ChangeLeverDynamic        bool `n:"动态修改杠杆"`
	GetTicker                 bool `n:"实现GetTickerByPair/Symbol同步接口"`
	GetTickerSignal           bool `n:"发送GetTicker Signal时会请求交易所"`
	GetLiteTickerSignal       bool `n:"发送GetListTicker Signal请求交易所轻量ticker"`
	PushOrderEventWhenLiq     bool `n:"发生adl/liq强平时\n向应用层发送cid随机的order event"`
	UnifiedPosClean           bool `n:"统一清仓流程\ntwap下单"`
	Partial                   bool `n:"增量订单簿"`
	DelayInTicker             bool `n:"ticker带有行情延迟信息"`
	FillRsEquityWhenGetEquity bool `n:"发GetEquity信号时会填写RsEquity结构体"`
	Standby                   bool `n:"RS实例可以对非主pair迅速下单"`
	WsSubWithRetry            bool `n:"WS实现重试订阅逻辑"`
	WsDepthLevel              bool `n:"根据交易所档位特性，自动订阅增量或全量订单簿"`
	WsResub                   bool `n:"长时间pub ws不推送，会重新订阅"`
	WsOrderEventMultiSource   bool `n:"orderevent有多种推送源头"`
	DoGetPriceLimit           bool `n:"支持rest获取限价信息"`
	GetPriWs                  bool `n:"rs中支持获取ws"`
	AutoRefillMargin          bool `n:"支持自动补充保证金"`

	DoPlaceOrderRsNor  bool `n:"DoPlaceOrderRsNor"`
	DoPlaceOrderRsColo bool `n:"DoPlaceOrderRsColo"`
	DoPlaceOrderWsNor  bool `n:"DoPlaceOrderWsNor"`
	DoPlaceOrderWsColo bool `n:"DoPlaceOrderWsColo"`

	DoCancelOrderRsNor  bool `n:"DoCancelOrderRsNor"`
	DoCancelOrderRsColo bool `n:"DoCancelOrderRsColo"`
	DoCancelOrderWsNor  bool `n:"DoCancelOrderWsNor"`
	DoCancelOrderWsColo bool `n:"DoCancelOrderWsColo"`

	DoAmendOrderRsNor  bool `n:"DoAmendOrderRsNor"`
	DoAmendOrderRsColo bool `n:"DoAmendOrderRsColo"`
	DoAmendOrderWsNor  bool `n:"DoAmendOrderWsNor"`
	DoAmendOrderWsColo bool `n:"DoAmendOrderWsColo"`
}

func EnsureIsRsFeatures(rs RsFeatures) {}
