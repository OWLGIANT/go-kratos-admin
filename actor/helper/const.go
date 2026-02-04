package helper

import (
	"fmt"
	"runtime"
	"strings"

	"actor/third/log"
)

type BrokerName int

/*
@所有人 另外各交易所接口命名规则 帮大家梳理如下：
【1】  {交易所名称}_spot   代表现货交易、币币交易     交易规则99%情况都是一致的
【2】  {交易所名称}_{类型}_swap  代表合约交易

这里的类型 同时和计价币和保证金币种关联 理论上有很多种组合方式构成不同类型的合约产品
但目前市场存活下来的 主流合约类型有3种 【linear】 【inverse】 【quanto】其中 linear是最主流的

1、以usdt usd 等为保证金的合约 通常叫正向合约（linear） 最常见 交易规则99%情况都是一致的
我们内部命名规范为 binance_usdt_swap phemex_usd_swap 等等

2、以btc eth 等为保证金的合约  通常叫反向合约  不同交易所规则不同
原来的命名方式为  binance_usd_swap  这个命名规则容易引起误解  但我们内部约定其表示为inverse类型的contract
比如 binance_inv_swap   表示计价币为 目标交易对的coin为保证金 无论是以btc eth还是其他币为保证金 交易规则是一致的 可以统一归为一类
特殊情况来了： 例如 bitmex交易所的 XBTUSD 和 ETHUSD ADAUSD等等
从名字上看他们似乎是同一类的 但是实际交易规则不一致 XBTUSD属于inverse合约 而ETHUSD等等属于quanto合约类型 在beastquant中 我们用 bitmex_qua_swap 来实现quanto类合约的交易

3、交割合约和相同保证金的永续合约交易规则一般是一致的 仅需要在pair 和 pairToSymol 中进行区分
beastquant内部的pair数据结构中有个more变量 用于描述具体交易的交割合约类型
*/
const (
	BrokernameBinanceUsdtSwap BrokerName = 1 + iota
	BrokernameBinanceUsdtSwapUm
	BrokernameBinanceUsdSwap
	BrokernameBinanceSpot

	BrokernameBingxSpot
	BrokernameBingxUsdtSwap

	BrokernameCoinexUsdtSwap
	BrokernameCoinexSpot

	BrokernameHuobiUsdtSwapIso
	BrokernameHuobiUsdtSwapUm
	BrokernameHuobiUsdtSwapUm2
	BrokernameHuobiUsdtSwap
	BrokernameHuobiUsdSwap
	BrokernameHuobiSpot

	BrokernameKucoinUsdtSwap
	BrokernameKucoinUsdSwap
	BrokernameKucoinSpot

	BrokernameGateUsdtSwap
	BrokernameGateUsdtSwapSpec
	BrokernameGateSpot

	BrokernameOkxUsdtSwap
	BrokernameOkxSpot

	BrokernameFtxUsdtSwap
	BrokernameFtxSpot

	BrokernameBitgetUsdtSwap
	BrokernameBitgetUsdtSwapV2
	BrokernameBitgetUsdtSwapSpec
	BrokernameBitgetSpot
	BrokernameBitgetSpotV2
	BrokernameBitgetSpotUm
	BrokernameBitgetUsdtUm
	BrokernameBitgetUsdtUmSpec

	BrokernameBybitUsdtSwap
	BrokernameBybitUsdtSwapV3
	BrokernameBybitSpot

	BrokernameBitmexUsdtSwap
	BrokernameBitmexInvSwap
	BrokernameBitmexSpot

	BrokernameBitmartUsdtSwap
	BrokernameBitmartSpot

	BrokernameBitfinexUsdtSwap
	BrokernameBitfinexUsdSwap
	BrokernameBitfinexSpot

	BrokernameUpbitKrwSwap
	BrokernameUpbitUsdtSwap
	BrokernameUpbitSpot

	BrokernameBitstampSpot
	BrokernameBitstampUsdtSwap

	BrokernameCoinbaseUsdtSwap
	BrokernameCoinbaseUsdcSwap
	BrokernameCoinbaseUsdSwap
	BrokernameCoinbaseSpot

	BrokernameKrakenSpot
	BrokernameKrakenUsdtSwap

	BrokernamePhemexSpot
	BrokernamePhemexUsdtSwap
	BrokernamePhemexUsdSwap

	BrokernameCryptoUsdtSwap
	BrokernameCryptoUsdSwap
	BrokernameCryptoSpot

	BrokernameAscendexUsdtSwap

	BrokernameLbankUsdtSwap

	BrokernameCoinsphSpot

	BrokernamePoloniexUsdtSwap

	BrokernameWooUsdtSwap

	BrokernameApexUsdtSwap
	BrokernameApexUsdcSwap
	BrokernameDydxUsdcSwap

	BrokernameAevoUsdcSwap

	BrokernameBackpackSpot

	BrokernameMexcSpot
	BrokernameMexcUsdtSwap

	BrokernameSuperexSpot

	BrokernameHashkeySpot
	BrokernameVertexUsdcSwap
	BrokernameHyperSpot
	BrokernameHyperUsdcSwap

	BrokernameMeteoraSpot
	BrokernameRaydiumSpot
	BrokernameRaydiumthirdSpot

	//
	BrokernameHyperEvmSpot
	BrokernameHyperEvmUsdcSwap
	BrokernameBitSpot
	BrokernameBitUsdtSwap

	BrokernamePionexUsdtSwap

	BrokernameCME
	BrokernameNASDAQ

	BrokernameOrangeSpot

	BrokernameUnknown // 必须放最后
)

var exBandAbbr = map[string]string{
	"bg":  "bitget",
	"bn":  "binance",
	"hb":  "huobi",
	"ok":  "okx",
	"ftx": "ftx",
	"cx":  "coinex",
	"bb":  "bybit",
	"bm":  "bitmart",
	"cp":  "crypto",
	"cb":  "coinbase",
	"kc":  "kucoin",
	"lb":  "lbank",
	"ray": "raydium",
	"up":  "upbit",
	"ph":  "phemex",
	"gt":  "gate",
	"bs":  "bitstamp",
	"mx":  "mexc",
	"hk":  "hashkey",
}

func (e BrokerName) StringAbbr() string {
	s := e.String()
	vals := strings.Split(s, ".")
	if len(vals) > 1 {
		vals[1] = strings.ReplaceAll(vals[1], "um", "uta")
	}
	vals[0] = strings.ReplaceAll(vals[0], "usdt_swap", "um")
	vals[0] = strings.ReplaceAll(vals[0], "usd_swap", "cm")
	for abbr, name := range exBandAbbr {
		if strings.HasPrefix(vals[0], name) {
			vals[0] = strings.Replace(vals[0], name, abbr, 1)
		}
	}
	return strings.Join(vals, ".")
}
func (e BrokerName) String() string {
	switch e {
	case BrokernameBingxSpot:
		return "bingx_spot"
	case BrokernameBingxUsdtSwap:
		return "bingx_usdt_swap"
	case BrokernameBinanceUsdtSwap:
		return "binance_usdt_swap"
	case BrokernameBinanceUsdtSwapUm:
		return "binance_usdt_swap.um"
	case BrokernameBinanceUsdSwap:
		return "binance_usd_swap"
	case BrokernameBinanceSpot:
		return "binance_spot"
	case BrokernameCoinexUsdtSwap:
		return "coinex_usdt_swap"
	case BrokernameCoinexSpot:
		return "coinex_spot"
	case BrokernameHuobiUsdtSwapIso:
		return "huobi_usdt_swap.iso"
	case BrokernameHuobiUsdtSwapUm:
		return "huobi_usdt_swap.um"
	case BrokernameHuobiUsdtSwapUm2:
		return "huobi_usdt_swap.um2"
	case BrokernameHuobiUsdtSwap:
		return "huobi_usdt_swap"
	case BrokernameHuobiUsdSwap:
		return "huobi_usd_swap"
	case BrokernameHuobiSpot:
		return "huobi_spot"
	case BrokernameKucoinUsdtSwap:
		return "kucoin_usdt_swap"
	case BrokernameKucoinUsdSwap:
		return "kucoin_usd_swap"
	case BrokernameKucoinSpot:
		return "kucoin_spot"
	case BrokernameGateUsdtSwap:
		return "gate_usdt_swap"
	case BrokernameGateUsdtSwapSpec:
		return "gate_usdt_swap.spec"
	case BrokernameGateSpot:
		return "gate_spot"
	case BrokernameOkxUsdtSwap:
		return "okx_usdt_swap"
	case BrokernameOkxSpot:
		return "okx_spot"
	case BrokernameFtxUsdtSwap:
		return "ftx_usdt_swap"
	case BrokernameFtxSpot:
		return "ftx_spot"
	case BrokernameBitgetUsdtSwap:
		return "bitget_usdt_swap"
	case BrokernameBitgetUsdtSwapV2:
		return "bitget_usdt_swap.v2"
	case BrokernameBitgetUsdtSwapSpec:
		return "bitget_usdt_swap.spec"
	case BrokernameBitgetSpot:
		return "bitget_spot"
	case BrokernameBitgetSpotV2:
		return "bitget_spot.v2"
	case BrokernameBitgetSpotUm:
		return "bitget_spot.um"
	case BrokernameBitgetUsdtUm:
		return "bitget_usdt_swap.um"
	case BrokernameBitgetUsdtUmSpec:
		return "bitget_usdt_swap.um.spec"
	case BrokernameBybitUsdtSwap:
		return "bybit_usdt_swap"
	case BrokernameBybitUsdtSwapV3:
		return "bybit_usdt_swap.v3"
	case BrokernameBybitSpot:
		return "bybit_spot"
	case BrokernameBitmexUsdtSwap:
		return "bitmex_usdt_swap"
	case BrokernameBitmexInvSwap:
		return "bitmex_inv_swap"
	case BrokernameBitmexSpot:
		return "bitmex_spot"
	case BrokernameBitmartUsdtSwap:
		return "bitmart_usdt_swap"
	case BrokernameBitmartSpot:
		return "bitmart_spot"
	case BrokernameCryptoUsdtSwap:
		return "crypto_usdt_swap"
	case BrokernameCryptoUsdSwap:
		return "crypto_usd_swap"
	case BrokernameCryptoSpot:
		return "crypto_spot"
	case BrokernameBitstampUsdtSwap:
		return "bitstamp_usdt_swap"
	case BrokernameBitstampSpot:
		return "bitstamp_spot"
	case BrokernameBitfinexUsdtSwap:
		return "bitfinex_usdt_swap"
	case BrokernameBitfinexUsdSwap:
		return "bitfinex_usd_swap"
	case BrokernameBitfinexSpot:
		return "bitfinex_spot"
	case BrokernamePhemexUsdtSwap:
		return "phemex_usdt_swap"
	case BrokernamePhemexUsdSwap:
		return "phemex_usd_swap"
	case BrokernamePhemexSpot:
		return "phemex_spot"
	case BrokernameKrakenUsdtSwap:
		return "kraken_usdt_swap"
	case BrokernameKrakenSpot:
		return "kraken_spot"
	case BrokernameCoinbaseUsdtSwap:
		return "coinbase_usdt_swap"
	case BrokernameCoinbaseUsdSwap:
		return "coinbase_usd_swap"
	case BrokernameCoinbaseSpot:
		return "coinbase_spot"
	case BrokernameUpbitUsdtSwap:
		return "upbit_usdt_swap"
	case BrokernameUpbitKrwSwap:
		return "upbit_krw_swap"
	case BrokernameUpbitSpot:
		return "upbit_spot"
	case BrokernameAscendexUsdtSwap:
		return "ascendex_usdt_swap"
	case BrokernameLbankUsdtSwap:
		return "lbank_usdt_swap"
	case BrokernameCoinsphSpot:
		return "coinsph_spot"
	case BrokernameCoinbaseUsdcSwap:
		return "coinbase_usdc_swap"
	case BrokernameWooUsdtSwap:
		return "woo_usdt_swap"
	case BrokernameApexUsdtSwap:
		return "apex_usdt_swap"
	case BrokernameApexUsdcSwap:
		return "apex_usdc_swap"
	case BrokernamePoloniexUsdtSwap:
		return "poloniex_usdt_swap"
	case BrokernameDydxUsdcSwap:
		return "dydx_usdc_swap"
	case BrokernameAevoUsdcSwap:
		return "aevo_usdc_swap"
	case BrokernameVertexUsdcSwap:
		return "vertex_usdc_swap"
	case BrokernameBackpackSpot:
		return "backpack_spot"
	case BrokernameMexcSpot:
		return "mexc_spot"
	case BrokernameMexcUsdtSwap:
		return "mexc_usdt_swap"
	case BrokernameSuperexSpot:
		return "superex_spot"
	case BrokernameHashkeySpot:
		return "hashkey_spot"
	case BrokernameHyperUsdcSwap:
		return "hyper_usdc_swap"
	case BrokernameHyperSpot:
		return "hyper_spot"
	case BrokernameHyperEvmUsdcSwap:
		return "hyperevm_usdc_swap"
	case BrokernameHyperEvmSpot:
		return "hyperevm_spot"
	case BrokernameMeteoraSpot:
		return "meteora_spot"
	case BrokernameRaydiumSpot:
		return "raydium_spot"
	case BrokernamePionexUsdtSwap:
		return "pionex_usdt_swap"
	case BrokernameRaydiumthirdSpot:
		return "raydiumthird_spot"
	case BrokernameBitSpot:
		return "bit_spot"
	case BrokernameBitUsdtSwap:
		return "bit_usdt_swap"
	case BrokernameCME:
		return "cme"
	case BrokernameNASDAQ:
		return "nasdaq"
	case BrokernameOrangeSpot:
		return "orange_spot"
	default:
		log.Errorf("[utf_ign]exchange not inited. %v", int(e))
		return "unknown exchange"
	}
}

func IsSpot(exName string) bool {
	return strings.Index(exName, "spot") > 0
}

func StringToBrokerName(e string) BrokerName {
	BROKER_IDX_START := 1
	for i := BROKER_IDX_START; i < int(BrokernameUnknown); i++ {
		broker := BrokerName(i)
		if broker.String() == e {
			return broker
		}
	}
	log.Errorf("exchange not inited %v", e)
	// 获取调用者信息，skip=1 表示获取直接调用 `printCallerInfo` 的函数信息
	_, file, line, ok := runtime.Caller(1)
	if ok {
		fmt.Printf("Caller: %s:%d\n", file, line)
	} else {
		fmt.Println("Failed to get caller info")
	}
	return BrokernameUnknown
}
func StringAbbrToBrokerName(e string) BrokerName {
	BROKER_IDX_START := 1
	for i := BROKER_IDX_START; i < int(BrokernameUnknown); i++ {
		broker := BrokerName(i)
		if broker.StringAbbr() == e {
			return broker
		}
	}
	log.Errorf("exchange not inited %v", e)
	// 获取调用者信息，skip=1 表示获取直接调用 `printCallerInfo` 的函数信息
	_, file, line, ok := runtime.Caller(1)
	if ok {
		fmt.Printf("Caller: %s:%d\n", file, line)
	} else {
		fmt.Println("Failed to get caller info")
	}
	return BrokernameUnknown
}

/* ------------------------------------------------------------------------------------------------------------------ */

// HandleMode 交易前处理方案
type HandleMode int

const (
	HandleModePrepare   = iota + 0 // 方案1 只获取基础交易信息 不处理仓位 会获取当前仓位 适合套利策略
	HandleModeCloseOne             // 方案2 只处理此交易对仓位 不会获取当前仓位 默认仓位清空 适合套利策略
	HandleModeCloseAll             // 方案3 处理全部交易对仓位 不会获取当前仓位 默认仓位清空 适合高频策略
	HandleModeCancelAll            // 方案4 取消所有挂单
	HandleModeCancelOne            // 方案5 仅撤销此交易对所有挂单
	HandleModePublic               // 不需要设置私有交易环境，只获取公共信息。设置exchange info和getticker就返回
)

const MAX_LEVERAGE = 5 // 所有交易所 未提供最大杠杆设置的都默认设置为5
