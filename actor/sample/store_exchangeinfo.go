package main

// 保存 exchangeinfo
// 项目根目录 go build  -tags=utf sample/main3.go

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"actor/helper"
	"actor/third/fixed"
)

func main() {
	domain()
}
func domain() {
	// for _, save := range []func() (string, helper.ExchangeInfo){save_bg_spot, save_bg_swap, save_bg_swap_v2, save_bg_spot_v2} {
	for _, save := range []func() (string, helper.ExchangeInfo){save_bg_swap, save_bg_swap_v2} {
		// for _, save := range []func() (string, helper.ExchangeInfo){save_bg_spot, save_bg_spot_v2} {
		exPair, info := save()
		content, err := json.Marshal(info)
		if err != nil {
			panic(err)
		}
		filename := "/tmp/" + exPair + ".json"
		os.WriteFile(filename, content, 0644)
		fmt.Println("have store. " + filename)
	}
}
func save_bg_spot() (exPair string, info helper.ExchangeInfo) {
	baseCoin := "usds"
	quoteCoin := "usdt"
	ticksize := 0.0001
	stepsize := 0.01
	minTradeNum := fixed.NewF(2)

	baseCoin = helper.Trim10Multiple(baseCoin)
	symbol := strings.ToUpper(baseCoin) + "USDT_SPBL"
	info2 := helper.ExchangeInfo{
		Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
		Symbol:         symbol,
		Status:         true,
		TickSize:       ticksize,
		StepSize:       stepsize,
		MaxOrderAmount: fixed.BIG,
		MinOrderAmount: minTradeNum,
		MaxOrderValue:  fixed.NewF(200000), // 限制bitget单手最大下单量 因为交易所有限制
		MinOrderValue:  fixed.TEN,
		Multi:          fixed.ONE,
		// 最大持仓价值
		MaxPosValue: fixed.BIG,
		// 最大持仓数量
		MaxPosAmount: fixed.BIG,
		// MaxLeverage:  helper.MAX_LEVERAGE,
		MaxLeverage: 3, // 这个所特殊，默认最大拉到10倍
	}
	if info2.MaxPosAmount == fixed.NaN || info2.MaxPosAmount.IsZero() {
		info2.MaxPosAmount = fixed.BIG
	}
	return "bitget_spot-" + baseCoin + "_" + quoteCoin, info2
}

func save_bg_spot_v2() (exPair string, info helper.ExchangeInfo) {
	baseCoin := "usds"
	quoteCoin := "usdt"
	ticksize := 0.0001
	stepsize := 0.01
	minTradeNum := fixed.NewF(2)

	baseCoin = helper.Trim10Multiple(baseCoin)
	symbol := strings.ToUpper(baseCoin) + "USDT_SPBL"
	info2 := helper.ExchangeInfo{
		Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
		Symbol:         symbol,
		Status:         true,
		TickSize:       ticksize,
		StepSize:       stepsize,
		MaxOrderAmount: fixed.BIG,
		MinOrderAmount: minTradeNum,
		MaxOrderValue:  fixed.NewF(200000), // 限制bitget单手最大下单量 因为交易所有限制
		MinOrderValue:  fixed.TEN,
		Multi:          fixed.ONE,
		// 最大持仓价值
		MaxPosValue: fixed.BIG,
		// 最大持仓数量
		MaxPosAmount: fixed.BIG,
		// MaxLeverage:  helper.MAX_LEVERAGE,
		MaxLeverage: 3, // 这个所特殊，默认最大拉到10倍
	}
	if info2.MaxPosAmount == fixed.NaN || info2.MaxPosAmount.IsZero() {
		info2.MaxPosAmount = fixed.BIG
	}
	return "bitget_spot.v2-" + baseCoin + "_" + quoteCoin, info2
}

func save_bg_swap() (exPair string, info helper.ExchangeInfo) {
	baseCoin := "bera"
	baseCoin = helper.Trim10Multiple(baseCoin)
	quoteCoin := "usdt"
	// pricePlace := 3
	// priceEndStep := 1
	// volumePlace := int64(1)
	minTradeAmount := fixed.NewF(0.1)
	minOrderValue := fixed.NewF(5)
	ticksize := 0.001
	stepsize := 0.1

	symbol := strings.ToUpper(baseCoin) + "USDT_UMCBL"
	// todo 这里可以获取到限价规则 按百分比上下浮动 还没想好怎么处理
	info2 := helper.ExchangeInfo{
		Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
		Symbol:         symbol,
		Status:         true,
		TickSize:       ticksize,
		StepSize:       stepsize,
		MaxOrderAmount: fixed.BIG,
		MinOrderAmount: minTradeAmount,
		MaxOrderValue:  fixed.NewF(200000), // 限制bitget单手最大下单量 因为交易所有限制
		MinOrderValue:  minOrderValue,
		Multi:          fixed.ONE,
		// 最大持仓价值
		MaxPosValue: fixed.BIG,
		// 最大持仓数量
		MaxPosAmount: fixed.BIG,
		// MaxLeverage:  helper.MAX_LEVERAGE,
		MaxLeverage: 3, // 这个所特殊，默认最大拉到10倍
	}
	if info2.MaxPosAmount == fixed.NaN || info2.MaxPosAmount.IsZero() {
		info2.MaxPosAmount = fixed.BIG
	}
	return "bitget_usdt_swap-" + baseCoin + "_" + quoteCoin, info2
}
func save_bg_swap_v2() (exPair string, info helper.ExchangeInfo) {
	baseCoin := "bera"
	baseCoin = helper.Trim10Multiple(baseCoin)
	quoteCoin := "usdt"
	minTradeAmount := fixed.NewF(0.1)
	minOrderValue := fixed.NewF(5)
	ticksize := 0.001
	stepsize := 0.1

	symbol := strings.ToUpper(baseCoin) + "USDT"
	// todo 这里可以获取到限价规则 按百分比上下浮动 还没想好怎么处理
	info2 := helper.ExchangeInfo{
		Pair:           helper.Pair{Base: baseCoin, Quote: quoteCoin},
		Symbol:         symbol,
		Status:         true,
		TickSize:       ticksize,
		StepSize:       stepsize,
		MaxOrderAmount: fixed.BIG,
		MinOrderAmount: minTradeAmount,
		MaxOrderValue:  fixed.NewF(200000), // 限制bitget单手最大下单量 因为交易所有限制
		MinOrderValue:  minOrderValue,
		Multi:          fixed.ONE,
		// 最大持仓价值
		MaxPosValue: fixed.BIG,
		// 最大持仓数量
		MaxPosAmount: fixed.BIG,
		// MaxLeverage:  helper.MAX_LEVERAGE,
		MaxLeverage: 3, // 这个所特殊，默认最大拉到10倍
	}
	if info2.MaxPosAmount == fixed.NaN || info2.MaxPosAmount.IsZero() {
		info2.MaxPosAmount = fixed.BIG
	}
	return "bitget_usdt_swap.v2-" + baseCoin + "_" + quoteCoin, info2
}
