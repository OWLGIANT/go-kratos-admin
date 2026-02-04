package base

func init() {
	// 为了能常量化和避免uint64溢出，这里只用63位
	if AbltEndBarrier > 63 {
		panic("Ability定义不应超过63种")
	}
}

// HasAbilities 判断交易所是否具备某些能力, error != nil表示无法判断，上层不应采纳，为nil才有价值
// 只要有一个能力不具备，就认为不具备，上层注意 abilities 切分
// func HasAbilities(abilitiedEx RsAbility, abilities TypeAbilitySet) (bool, error) {
func HasAbilities(abilitiedEx RsAbility, abilities TypeAbilitySet) (bool, error) {
	// 明确不具备就是不具备，优先级最高。
	if abilitiedEx.GetExcludeAbilities()&abilities != 0 {
		return false, nil
	}

	// 类别层级判断 start
	// 不具备整个大类功能，就不具备小类功能
	//---------------------
	if abilitiedEx.GetExcludeAbilities()&AbltWsPri != 0 {
		if abilities&(     //
		AbltWsPriReqOrder| //
			AbltWsPriPosition| //
			AbltWsPriPositionWithSeq| //
			AbltWsPriEquity| //
			AbltWsPriEquityAvailReducedWhenHold| //
			AbltWsPriEquityWithUpnl| //
			AbltWsPriEquityWithSeq| //
			AbltWsPriEquityWithFree| //
			AbltWsPriOrder| //
			AbltWsPriOrderFilledPriceCorrect| //
			AbltWsPriPosFasterOrder| //
			AbltWsPriOrderTypeExact) != 0 {
			return false, nil
		}
	}
	if abilitiedEx.GetExcludeAbilities()&AbltWsPriEquity != 0 {
		if abilities&(                       //
		AbltWsPriEquityAvailReducedWhenHold| //
			AbltWsPriEquityWithSeq| //
			AbltWsPriEquityWithUpnl| //
			AbltWsPriEquityWithFree) != 0 {
			return false, nil
		}
	}
	if abilitiedEx.GetExcludeAbilities()&AbltOrderCid != 0 {
		if abilities&(        //
		AbltOrderCancelByCid| //
			AbltRsPriCheckOrderByCid) != 0 {
			return false, nil
		}
	}

	// 类别层级判断 end

	if abilitiedEx.GetExcludeAbilities()&AbltRsPriCheckOrder != 0 {
		if abilities&(AbltRsPriCheckOrder|AbltRsPriCheckOrderByCid) != 0 {
			return false, nil
		}
	}
	//---------------------

	if abilitiedEx.GetIncludeAbilities()&abilities == abilities {
		return true, nil
	}
	return false, nil
}

// 从连接类型和功能领域两个角度做大类分类，大类之下可以灵活添加小类
const (
	AbltNil = 0
	// === 连接类型
	// Rest Pub
	AbltRsPub                 = 1 << iota
	AbltRsPubGetTickerWithSeq = 1 << iota // 带交易所seq

	// Rest Pri
	AbltRsPri                  = 1 << iota
	AbltRsPriSetLeverage       = 1 << iota // 能在 rest api 设置杠杆
	AbltRsPriCheckOrder        = 1 << iota // 能在 rest api 检查订单，部分奇葩交易所不支持
	AbltRsPriCheckOrderByCid   = 1 << iota // 能在 rest api 通过client order 检查订单，部分交易所不支持，例如gate
	AbltRsPriGetPos            = 1 << iota
	AbltRsPriGetPosWithSeq     = 1 << iota // 交易所返回跟ws pos中同一意义的seq
	AbltRsPriGetEquityWithSeq  = 1 << iota // 交易所返回跟ws equit中同一意义的seq
	AbltRsPriOrderFilledPrice  = 1 << iota // 能从rs获取avg filled price
	AbltRsPriGetAllIndex       = 1 << iota // 能在 rest api 设置杠杆
	AbltRsPriGetAllFundingRate = 1 << iota // 能在 rest api 设置杠杆

	// Ws Pub
	AbltWsPub                  = 1 << iota
	AbltWsPubTrade             = 1 << iota // ws pub 能订阅 trade 数据
	AbltWsPubPartial           = 1 << iota // ws pub 能订阅 增量订单薄 数据
	AbltWsPubTicker100PCorrect = 1 << iota // ws pub ticker 是否100%准确， 带seq或检验的才可以
	AbltWsPubTickerWithSeq     = 1 << iota // 带交易所seq

	// Ws Pri.
	// important!!! 如增加， 请在 HasAbilities 里增加小类判断
	AbltWsPri                             = 1 << iota // 大类
	AbltWsPriReqOrder                     = 1 << iota // 能在 ws 实例里发起 order 请求
	AbltWsPriPosition                     = 1 << iota // 推送事件
	AbltWsPriPositionWithSeq              = 1 << iota
	AbltWsPriPositionHasSeqSameAsTicker   = 1 << iota // position中有跟ticker相同的seq
	AbltWsPriEquity                       = 1 << iota // 推送事件
	AbltWsPriEquityAvailReducedWhenHold   = 1 << iota // 持仓时(现货或合约都适用)，推送事件中可用资金会减少
	AbltWsPriEquityWithUpnl               = 1 << iota // 推送事件中，总资金包含浮盈浮亏
	AbltWsPriEquityWithSeq                = 1 << iota
	AbltWsPriEquityWithFree               = 1 << iota // 推送带有可用余额
	AbltWsPriOrder                        = 1 << iota // 推送事件
	AbltWsPriOrderFilledPriceCorrect      = 1 << iota // order事件有成交价且正确而不是相似
	AbltWsPriOrderFee                     = 1 << iota // 订单推送中带有手续费
	AbltWsPriOrderTypeExact               = 1 << iota // order事件能精确表达订单类型，有些所会有缺失tif\poc\ioc等，例如kucoin hf spot
	AbltWsPriPosFasterOrder               = 1 << iota // 成交事件推送时，pos 比 order更快
	AbltWsPriOrderEventHasSeqSameAsTicker = 1 << iota // order事件中有跟ticker相同的seq

	// === 功能领域
	// Equity
	AbltEquityAvailReducedWhenPendingOrder = 1 << iota // 挂单时，可用资金会减少
	AbltEquityAvailReducedWhenHold         = 1 << iota // 持仓时，rest获取的可用资金会减少

	// Order
	AbltOrderAmend           = 1 << iota // 修改订单，含rest ws
	AbltOrderAmendByCid      = 1 << iota // 能通过cid修改订单，含rest ws
	AbltOrderCid             = 1 << iota // 订单信息包含client id
	AbltOrderCancelByCid     = 1 << iota // 能通过cid取消订单，含rest ws
	AbltOrderExceedLimitBuy  = 1 << iota // 能挂超价卖单
	AbltOrderExceedLimitSell = 1 << iota // 能挂超价卖单
	AbltOrderMarketBuy       = 1 << iota // 能市价买入， lbank swap不支持
	AbltOrderMarketSell      = 1 << iota // 能市价卖出， apex swap不支持
	AbltOrderIoc             = 1 << iota // ioc order

	// Position
	AbltPositionModeHedge  = 1 << iota // 双向持仓
	AbltPositionModeOneWay = 1 << iota // 单向持仓

	// Trade

	// Margin 保证金
	AbltMarginModeCrossed  = 1 << iota
	AbltMarginModeIsolated = 1 << iota

	AbltEndBarrier = iota
)

const ALL_ABILITIES TypeAbilitySet = (1 << AbltEndBarrier) - 1

// 先默认都没有，日后再默认都有 kc@2023年11月13日
const ABILITIES_SEQ_RS = AbltRsPriGetEquityWithSeq | AbltRsPriGetPosWithSeq | AbltRsPubGetTickerWithSeq
const ABILITIES_SEQ_WS = AbltWsPriPositionWithSeq | AbltWsPriEquityWithSeq | AbltWsPubTicker100PCorrect
const ABILITIES_SEQ = ABILITIES_SEQ_RS | ABILITIES_SEQ_WS

// 大部分所没有的放这里
const DEFAULT_HAS_NO = AbltWsPriOrderEventHasSeqSameAsTicker ^ AbltWsPriOrderFee ^ AbltRsPriGetAllIndex ^ AbltRsPriGetAllFundingRate ^ AbltOrderAmend ^ AbltOrderAmendByCid

const ABILITIES_ONLY_SWAP_HAVE = AbltPositionModeHedge | AbltPositionModeOneWay | AbltWsPriPosition

const DEFAULT_ABILITIES_SWAP TypeAbilitySet = ALL_ABILITIES ^ AbltWsPriReqOrder ^ ABILITIES_SEQ ^ AbltWsPriPosFasterOrder ^ DEFAULT_HAS_NO
const DEFAULT_ABILITIES_SPOT TypeAbilitySet = ALL_ABILITIES ^ AbltWsPriReqOrder ^ ABILITIES_ONLY_SWAP_HAVE ^ ABILITIES_SEQ ^ AbltWsPriPosFasterOrder ^ DEFAULT_HAS_NO

type TypeAbility uint64
type TypeAbilitySet uint64 // bits map设计

// RsAbility 交易所能力.
// 交易所实现时，一般是在Include定义一个超集，Exclude再添加不支持的功能
// 位运算: ^是减去某能力，|是添加某能力
type RsAbility interface {
	// 交易所不具备的能力, 优先级比Include高
	GetExcludeAbilities() TypeAbilitySet
	// 交易所具备的能力, 一般返回 DEFAULT_ABILITIES
	GetIncludeAbilities() TypeAbilitySet
}
