package helper

import (
	"fmt"

	"actor/third/log"
)

// ReqLimitRules 限频规则集合
type ReqLimitRules struct {
	Name   BrokerName        // 交易所名称
	Mode   ReqLimitMode      // 限频方式
	Mono   ReqLimitRuleMono  // 独占模式下的限频规则
	Share  ReqLimitRuleShare // 共享模式下的限频规则
	Range  ReqLimitRange     // 限频范围
	Text   string            // 限频规则描述 用于记录一些特殊的限频规则
	DocUrl string            // 限频规则文档地址
}

func (r ReqLimitRules) String() string {
	return fmt.Sprintf("[%v] 限频方式:%v \n 限频范围:%v \n 独占限频规则:%v 共享限频规则:%v \n"+
		" 限频文字备注:%v \n 限频规则文档地址:%v \n",
		r.Name, r.Mode, r.Range, r.Mono, r.Share, r.Text, r.DocUrl)
}

// 限频方式
type ReqLimitMode int

const (
	// 共享模式 下单撤单共享频率
	ReqLimitModeShare ReqLimitMode = 0 + iota
	// 独占模式 下单撤单各自独占频率
	ReqLimitModeMono
	// 未知模式
	reqLimitModeUnknown
)

// 限频范围
type ReqLimitRange int

const (
	// 按uid 全symbol限频
	ReqLimitRangeGlobal ReqLimitRange = 0 + iota
	// 按uid 各symbol单独限频
	ReqLimitRangeSymbol
	// 母子账号共同限频 按uid 反人类设计
	ReqLimitRangeMasterGlobal
	// 母子账号共同限频 按symbol 反人类设计
	ReqLimitRangeMasterSymbol
)

// 共享模式下的限频规则
type ReqLimitRuleShare struct {
	Num      float64 // 订单接口限频次数 单位次
	Interval float64 // 订单接口限频间隔 单位毫秒
}

// 独占模式下的限频规则
type ReqLimitRuleMono struct {
	PlaceNum       float64 // 下单限频次数 单位次
	PlaceInterval  float64 // 下单限频间隔 单位毫秒
	CancelNum      float64 // 撤单限频次数 单位次
	CancelInterval float64 // 撤单限频间隔 单位毫秒
}

/*------------------------------------------------------------------------------------------------------------------*/
// 存放所有的默认限频规则
// GetReqLimit 获取不同交易所的本地限频规则 n次每秒 n必须>=1
// 表示该交易所在1秒内 允许下n笔订单 策略层面获取该数值 对sendsignal中下单频率进行相应限制
// 填8成值，留点buffer ，例如 10*0.8
// 不建议小于4 一般来说策略需要支持在一个限频周期内同时报单至少多空各一笔 否则可能会不支持偏高频的交易
// 此处的频率 仅代表下单频率 一般来说 下单频率是木桶最短的一块板子 如果保证下单符合要求 其他接口一般都符合要求
// 如果交易所按1秒周期限频 我们内部建议按500ms周期限频 这样才可以确保全局不超频率
// Input 交易所名称
// output 内部限频规则
func GetReqLimit(exchangeName string) ReqLimitRules {
	switch exchangeName {
	// coinex 增加24h母子账户总60,000,000的总下单请求， 2024-3-7 生效 。by kc/sia
	// 2024-3-16，母子账户一小时600w
	case BrokernameCoinexUsdtSwap.String(): // 2月底 限频砍半 https://viabtc.github.io/coinex_api_tw_doc/general/#docsgeneral004_common001_api_brief_information 这里所有的接口都砍半
		return ReqLimitRules{ // 限频 20/1s 下单和撤单 共享 限频
			Name: BrokernameCoinexUsdtSwap,
			Mode: ReqLimitModeMono,
			Mono: ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 10000},
			// Share: ReqLimitRuleShare{Num: 20, Interval: 1000},
			Range: ReqLimitRangeGlobal,
			Text:  "母子账户一小时600w, 400 sub-accounts",
		}
	case BrokernameVertexUsdcSwap.String(): //https://docs.vertexprotocol.com/developer-resources/api/gateway/executes/place-order
		return ReqLimitRules{
			Name:  BrokernameCoinexUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Range: ReqLimitRangeGlobal,
			Text:  "",
		}
	case BrokernameCoinexSpot.String(): // 2月底 限频砍半 https://viabtc.github.io/coinex_api_tw_doc/general/#docsgeneral004_common001_api_brief_information 这里所有的接口都砍半
		return ReqLimitRules{ // 限频 20/1s 下单和撤单 共享 限频
			Name:  BrokernameCoinexSpot,
			Mode:  ReqLimitModeShare,
			Mono:  ReqLimitRuleMono{},
			Share: ReqLimitRuleShare{Num: 20, Interval: 1000},
			Range: ReqLimitRangeGlobal,
			Text:  "coinex 增加24h总60,000,000的总下单请求 相当于1秒700单",
		}
	case BrokernameBingxUsdtSwap.String(): // https://bingx-api.github.io/docs/#/en-us/swapV2/base-info.html#Rate%20limit
		return ReqLimitRules{ // 限频 5/1s 共享 限频, 5 per s by uid. 100 per 10s by Ip
			Name:  BrokernameBingxUsdtSwap,
			Mode:  ReqLimitModeShare,
			Mono:  ReqLimitRuleMono{},
			Share: ReqLimitRuleShare{Num: 2, Interval: 1000}, // 实际容易触发，调更低
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetUsdtSwap.String(): // https://bitgetlimited.github.io/apidoc/zh/mix/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetUsdtSwapV2.String(): // https://bitgetlimited.github.io/apidoc/zh/mix/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetUsdtSwapV2,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetUsdtUm.String(): // https://bitgetlimited.github.io/apidoc/zh/mix/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetUsdtUm,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetUsdtSwapSpec.String(), BrokernameBitgetUsdtUmSpec.String(): // https://bitgetlimited.github.io/apidoc/zh/mix/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetUsdtSwapSpec,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 3, PlaceInterval: 1000, CancelNum: 3, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetSpot.String(): // https://bitgetlimited.github.io/apidoc/zh/spot/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetSpotV2.String():
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetSpotV2,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitgetSpotUm.String(): // https://bitgetlimited.github.io/apidoc/zh/mix/#fd6ce2a756 小心biget有母子账号总限频的隐藏限制
		return ReqLimitRules{ // 限频 10/1s 下单和撤单 分别 限频
			Name:  BrokernameBitgetSpotUm,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 3, PlaceInterval: 1000, CancelNum: 3, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHuobiUsdtSwap.String(): // huobi usdt swap 限频 6/1s https://huobiapi.github.io/docs/usdt_swap/v1/cn/#4977e9e52a
		// 普通用户，需要密钥的私有接口，每个UID 3秒最多 144 次请求(交易接口3秒最多 72 次请求，查询接口3秒最多 72 次请求)
		// 交割合约、币本位永续合约和U本位合约都分开限频。
		return ReqLimitRules{ // 限频 144/3s 下单和撤单共享限频
			Name:  BrokernameHuobiUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 6, PlaceInterval: 1000, CancelNum: 6, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHuobiUsdtSwapUm.String():
		return ReqLimitRules{
			Name:  BrokernameHuobiUsdtSwapUm,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 6, PlaceInterval: 1000, CancelNum: 6, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHuobiUsdtSwapUm2.String():
		return ReqLimitRules{
			Name:  BrokernameHuobiUsdtSwapUm2,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 6, PlaceInterval: 1000, CancelNum: 6, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHuobiSpot.String(): // binance spot 限频 10/1s https://huobiapi.github.io/docs/spot/v1/cn/#ec3cea3958
		//新限频规则 todo 接口组完善
		//新限频规则采用基于UID的限频机制，即，同一UID下各API Key同时对某单个节点请求的频率不能超出单位时间内该节点最大允许访问次数的限制
		//用户可根据Http Header中的"X-HB-RateLimit-Requests-Remain"（限频剩余次数）及"X-HB-RateLimit-Requests-Expire"（窗口过期时间）查看当前限频使用情况，以及所在时间窗口的过期时间，根据该数值动态调整您的请求频率。
		return ReqLimitRules{ // todo 交易所有新的限频规则上线 待接口组完善
			Name:  BrokernameHuobiSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 6, PlaceInterval: 1000, CancelNum: 6, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameGateSpot.String(): // gate spot 限频 10/1s https://www.gate.io/docs/developers/apiv4/zh_CN/#%E9%99%90%E9%A2%91%E8%A7%84%E5%88%99
		//现货批量/单个下单/单个修改接口一共 500r/10s(uid), 订单数10单/s (uid+市场)
		//现货批量/单个撤单接口一共 750r/10s
		//现货其他单个接口 200r/10s
		return ReqLimitRules{
			Name:  BrokernameGateSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameGateUsdtSwap.String(): // https://www.gate.io/docs/developers/apiv4/zh_CN/#%E9%99%90%E9%A2%91%E8%A7%84%E5%88%99
		//合约批量/单个下单接口一共 500r/10s
		//合约批量/单个撤单接口一共 750r/10s
		//永续合约其他单个接口 200r/10s
		return ReqLimitRules{
			Name:  BrokernameGateUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameGateUsdtSwapSpec.String():
		return ReqLimitRules{
			Name:  BrokernameGateUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameKucoinUsdtSwap.String(): // https://www.kucoin.com/zh-hant/docs/basic-info/request-rate-limit/rest-api 更新v2限频规则
		return ReqLimitRules{
			Name:  BrokernameKucoinUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 20, PlaceInterval: 1000, CancelNum: 20, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameKucoinSpot.String(): // uid级别 150/3s, https://docs.kucoin.com/spot-hf/cn/#a9d0f69ef1
		return ReqLimitRules{
			Name:  BrokernameKucoinSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 20, PlaceInterval: 1000, CancelNum: 20, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBinanceSpot.String(): // https://binance-docs.github.io/apidocs/spot/cn/#12907e94be  exchangeInfo接口有定义
		return ReqLimitRules{
			Name:  BrokernameBinanceSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBinanceUsdtSwap.String(): // https://binance-docs.github.io/apidocs/futures/cn/#12907e94be 没有给出明确定义 需要从header中获取 自我严格克制
		return ReqLimitRules{
			Name:  BrokernameBinanceUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 20, PlaceInterval: 1000, CancelNum: 20, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBinanceUsdSwap.String():
		return ReqLimitRules{
			Name:  BrokernameBinanceUsdSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 20, PlaceInterval: 1000, CancelNum: 20, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBinanceUsdtSwapUm.String():
		return ReqLimitRules{
			Name:  BrokernameBinanceUsdtSwapUm,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 20, PlaceInterval: 1000, CancelNum: 20, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameOkxUsdtSwap.String(): // https://www.okx.com/docs-v5/zh/#rest-api-trade-place-order
		return ReqLimitRules{
			Name:  BrokernameOkxUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 30, PlaceInterval: 1000, CancelNum: 30, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameOkxSpot.String():
		return ReqLimitRules{
			Name:  BrokernameOkxSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 30, PlaceInterval: 1000, CancelNum: 30, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	// case BrokernameBybitUsdtSwap.String(): // https://bybit-exchange.github.io/docs/zh-TW/derivatives/rate-limit
	// return 10
	// https://bybit-exchange.github.io/docs/v5/rate-limit
	case BrokernameBybitUsdtSwap.String(): // 特殊调整到120/s 但是实际用不到这么高
		// 120/s per ip，所有请求，不区分api接口
		return ReqLimitRules{
			Name:  BrokernameBybitUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 40, PlaceInterval: 1000, CancelNum: 40, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBybitSpot.String(): // 特殊调整到120/s 但是实际用不到这么高
		return ReqLimitRules{
			Name:  BrokernameBybitSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 40, PlaceInterval: 1000, CancelNum: 40, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitmartUsdtSwap.String(): // https://developer-pro.bitmart.com/en/futures/#rate-limit
		return ReqLimitRules{
			Name:  BrokernameBitmartUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitmartSpot.String(): // https://developer-pro.bitmart.com/en/spot/#rate-limit
		return ReqLimitRules{
			Name:  BrokernameBitmartSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 25, PlaceInterval: 1000, CancelNum: 25, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitmexUsdtSwap.String():
		return ReqLimitRules{
			Name:  BrokernameBitmexUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 5, PlaceInterval: 1000, CancelNum: 5, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitmexInvSwap.String():
		return ReqLimitRules{
			Name:  BrokernameBitmexInvSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 5, PlaceInterval: 1000, CancelNum: 5, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernamePhemexUsdtSwap.String(): // https://phemex-docs.github.io/#ip-ratelimits
		return ReqLimitRules{
			Name:  BrokernamePhemexUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernamePhemexUsdSwap.String(): // https://phemex-docs.github.io/#ip-ratelimits
		return ReqLimitRules{
			Name:  BrokernamePhemexUsdSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernamePhemexSpot.String(): // https://phemex-docs.github.io/#ip-ratelimits
		return ReqLimitRules{
			Name:  BrokernamePhemexSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameAscendexUsdtSwap.String(): // https://ascendex.github.io/ascendex-futures-pro-api-v2/#risk-limit-info-v2
		// 看其他都偏小点？8000 / 60 / 2
		return ReqLimitRules{
			Name:  BrokernameAscendexUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 40, PlaceInterval: 1000, CancelNum: 40, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameUpbitSpot.String(): // https://docs.upbit.com/docs/user-request-guide
		return ReqLimitRules{
			Name:  BrokernameUpbitSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernamePoloniexUsdtSwap.String():
		// place limit 在 header中;cancel limit: batch 1/1s, single 10/1s
		return ReqLimitRules{
			Name:  BrokernamePoloniexUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameLbankUsdtSwap.String(): // 交易所技术员: "加完白名单之后，比较特殊的走特殊配置，每一个不一样，常规的是10秒20000次"
		return ReqLimitRules{
			Name:  BrokernameLbankUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 100, PlaceInterval: 1000, CancelNum: 100, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
		// TODO coinsph https://coins-docs.github.io/rest-api/#limits
	case BrokernameCoinbaseUsdcSwap.String(): // uid级别 150/3s, https://docs.kucoin.com/spot-hf/cn/#a9d0f69ef1
		return ReqLimitRules{
			Name:  BrokernameCoinbaseUsdcSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 6, PlaceInterval: 1000, CancelNum: 6, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameWooUsdtSwap.String(): // https://docs.woo.org/#send-order
		return ReqLimitRules{
			Name:  BrokernameWooUsdtSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 4, PlaceInterval: 1000, CancelNum: 4, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameApexUsdtSwap.String():
		// 官方文档是80单 post/60秒。但我们系统必须>=1，触发限频先容忍 https://api-docs.pro.apex.exchange/#general-rate-limits
		return ReqLimitRules{
			Name:   BrokernameApexUsdtSwap,
			Mode:   ReqLimitModeMono,
			Mono:   ReqLimitRuleMono{PlaceNum: 2, PlaceInterval: 1000, CancelNum: 2, CancelInterval: 1000},
			Share:  ReqLimitRuleShare{},
			Range:  ReqLimitRangeGlobal,
			DocUrl: "https://api-docs.pro.apex.exchange/#general-rate-limits",
		}
	case BrokernameApexUsdcSwap.String():
		// 官方文档是80单 post/60秒。但我们系统必须>=1，触发限频先容忍 https://api-docs.pro.apex.exchange/#general-rate-limits
		return ReqLimitRules{
			Name:   BrokernameApexUsdcSwap,
			Mode:   ReqLimitModeMono,
			Mono:   ReqLimitRuleMono{PlaceNum: 2, PlaceInterval: 1000, CancelNum: 2, CancelInterval: 1000},
			Share:  ReqLimitRuleShare{},
			Range:  ReqLimitRangeGlobal,
			DocUrl: "https://api-docs.pro.apex.exchange/#general-rate-limits",
		}
	case BrokernameDydxUsdcSwap.String():
		// 官方文档是80单 post/60秒。但我们系统必须>=1，触发限频先容忍
		return ReqLimitRules{
			Name:  BrokernameDydxUsdcSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 2, PlaceInterval: 1000, CancelNum: 2, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameAevoUsdcSwap.String():
		// 官网没写具体多少，根据经验，先用5
		return ReqLimitRules{
			Name:  BrokernameAevoUsdcSwap,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 2, PlaceInterval: 1000, CancelNum: 2, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameMexcSpot.String(): // 2月底 限频砍半 https://viabtc.github.io/coinex_api_tw_doc/general/#docsgeneral004_common001_api_brief_information 这里所有的接口都砍半
		return ReqLimitRules{ // 限频 20/1s 下单和撤单 共享 限频
			Name:  BrokernameMexcSpot,
			Mode:  ReqLimitModeShare,
			Mono:  ReqLimitRuleMono{},
			Share: ReqLimitRuleShare{Num: 40, Interval: 1000},
			Range: ReqLimitRangeGlobal,
			Text:  "",
		}
	case BrokernameSuperexSpot.String():
		return ReqLimitRules{
			Name:  BrokernameSuperexSpot,
			Mode:  ReqLimitModeShare,
			Mono:  ReqLimitRuleMono{},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
			Text:  "",
		}

	case BrokernameHashkeySpot.String(): // hashkey spot 查询限频  2/1s  下单限频 10/1s https://hashkeyglobal-apidoc.readme.io/reference/preparations#order-rate-limiting
		// Unless explicitly stated otherwise, each API Key has a default rate limit of 2 requests per second for query-related endpoints, while order-related endpoints allow 10 requests per second.
		//现货批量/单个下单/单个修改接口一共 500r/10s(uid), 订单数10单/s (uid+市场)
		//现货批量/单个撤单接口一共 750r/10s
		//现货其他单个接口 200r/10s

		return ReqLimitRules{
			Name: BrokernameHashkeySpot,
			Mode: ReqLimitModeShare,
			// Mono:  ReqLimitRuleMono{PlaceNum: 5, PlaceInterval: 1000, CancelNum: 5, CancelInterval: 1000},
			Share: ReqLimitRuleShare{Num: 8, Interval: 1000},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHyperSpot.String():
		// REST requests share an aggregated weight limit of 1200 per minute.
		// 批量下单权重 1 + floor(batch_length / 40)
		// info接口权重2
		return ReqLimitRules{
			Name: BrokernameHyperSpot,
			Mode: ReqLimitModeShare,
			// 1200/60/2
			Share: ReqLimitRuleShare{Num: 10, Interval: 1000},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameHyperUsdcSwap.String():
		// REST requests share an aggregated weight limit of 1200 per minute.
		// 批量下单权重 1 + floor(batch_length / 40)
		// info接口权重2
		return ReqLimitRules{
			Name: BrokernameHyperUsdcSwap,
			Mode: ReqLimitModeShare,
			// 1200/60/2
			Share: ReqLimitRuleShare{Num: 10, Interval: 1000},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitSpot.String():
		// REST requests share an aggregated weight limit of 1200 per minute.
		// 批量下单权重 1 + floor(batch_length / 40)
		// info接口权重2
		return ReqLimitRules{
			Name: BrokernameBitSpot,
			Mode: ReqLimitModeShare,
			// 1200/60/2
			Share: ReqLimitRuleShare{Num: 5, Interval: 1000},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameBitUsdtSwap.String():
		// REST requests share an aggregated weight limit of 1200 per minute.
		// 批量下单权重 1 + floor(batch_length / 40)
		// info接口权重2
		return ReqLimitRules{
			Name: BrokernameBitUsdtSwap,
			Mode: ReqLimitModeShare,
			// 1200/60/2
			Share: ReqLimitRuleShare{Num: 5, Interval: 1000},
			Range: ReqLimitRangeGlobal,
		}
	case BrokernameOrangeSpot.String():
		return ReqLimitRules{
			Name:  BrokernameOrangeSpot,
			Mode:  ReqLimitModeMono,
			Mono:  ReqLimitRuleMono{PlaceNum: 10, PlaceInterval: 1000, CancelNum: 10, CancelInterval: 1000},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	default:
		log.Errorf("未查询到预设限频规则:%v ***需要人工处理***", exchangeName)
		return ReqLimitRules{
			Name:  BrokernameUnknown,
			Mode:  reqLimitModeUnknown,
			Mono:  ReqLimitRuleMono{},
			Share: ReqLimitRuleShare{},
			Range: ReqLimitRangeGlobal,
		}
	}
}
