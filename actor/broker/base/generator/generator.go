// Generator 负责生成所有接口实例
package generator

import (
	"actor/broker/ex/binance_spot"
	"actor/broker/ex/binance_usdt_swap"
	"actor/broker/ex/bitget_spot"
	"actor/broker/ex/bitget_usdt_swap"
	"actor/broker/ex/okx_spot"
	"actor/broker/ex/okx_usdt_swap"
	"fmt"
	"strings"

	"actor/broker/base"
	"actor/helper"
)

type GeneratorRsType func(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs
type GeneratorWsType func(params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws

type g struct {
	Rs GeneratorRsType
	Ws GeneratorWsType
}

var Generators map[string]g

func init() {
	Generators = make(map[string]g)
	Generators[helper.BrokernameOkxSpot.String()] = g{Rs: okx_spot.NewRs, Ws: okx_spot.NewWs}
	Generators[helper.BrokernameOkxUsdtSwap.String()] = g{Rs: okx_usdt_swap.NewRs, Ws: okx_usdt_swap.NewWs}
	Generators[helper.BrokernameBinanceSpot.String()] = g{Rs: binance_spot.NewRs, Ws: binance_spot.NewWs}
	Generators[helper.BrokernameBinanceUsdtSwap.String()] = g{Rs: binance_usdt_swap.NewRs, Ws: binance_usdt_swap.NewWs}
	Generators[helper.BrokernameBitgetSpot.String()] = g{Rs: bitget_spot.NewRs, Ws: bitget_spot.NewWs}
	Generators[helper.BrokernameBitgetUsdtSwap.String()] = g{Rs: bitget_usdt_swap.NewRs, Ws: bitget_usdt_swap.NewWs}
}

// AllowRsClient 返回是否允许该盘口
func AllowRsClient(name string) bool {
	val, exist := Generators[name]
	return exist && val.Rs != nil
}

func GetAllowExchanges() []string {
	res := make([]string, 0, len(Generators))
	for k := range Generators {
		res = append(res, k)
	}
	return res
}

func GetTestingExchanges(slient ...bool) []string {
	lastTradingExchanges := []string{
		// "apex_usdt_swap",
		// "apex_usdc_swap",
		"binance_usdt_swap",
		"binance_usd_swap",
		"bitget_usdt_swap",
		"bit_usdt_swap",
		"bitmart_usdt_swap",
		"bybit_usdt_swap",
		// "bybit_usdt_swap.v3",
		"coinex_usdt_swap",
		"coinbase_usdc_swap",
		"upbit_spot",
		// "dydx_usdc_swap",
		"gate_usdt_swap",
		"huobi_usdt_swap",
		"kucoin_usdt_swap",
		"okx_usdt_swap",
		"phemex_usdt_swap",
		"woo_usdt_swap",
		//
		"binance_spot",
		"bitget_spot",
		"bitmart_spot",
		"bybit_spot",
		"coinex_spot",
		"gate_spot",
		"huobi_spot",
		"kucoin_spot",
		"okx_spot",
		// "bitget_usdt_swap.um",
	}
	lastTradingExchanges = base.SortExNames(lastTradingExchanges)
	if slient == nil || len(slient) == 0 {
		// 这段输出不能随便改，有自动化工具依赖
		fmt.Printf("lastTradingExchanges:%s\n", strings.Join(lastTradingExchanges, ","))
	}
	return lastTradingExchanges
}

// AllowWsClient 返回是否允许该盘口
func AllowWsClient(name string) bool {
	val, exist := Generators[name]
	return exist && val.Ws != nil
}

// GenerateRs 生成一个rs实例
func GenerateRs(name string, params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Rs {
	if generator, exist := Generators[name]; exist && generator.Rs != nil {
		return generator.Rs(params, msg, pairInfo, cb)
	}
	return nil
}

// GenerateWs 生成一个ws实例
func GenerateWs(name string, params *helper.BrokerConfigExt, msg *helper.TradeMsg, pairInfo *helper.ExchangeInfo, cb helper.CallbackFunc) base.Ws {
	if generator, exist := Generators[name]; exist && generator.Ws != nil {
		return generator.Ws(params, msg, pairInfo, cb)
	}
	return nil
}
