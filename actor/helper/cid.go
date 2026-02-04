package helper

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
)

/*
	使用方法
	策略启动时
	cidTool := helper.NewCidTool("huobi_usdt_swap", "1")
	每次需要cid时
	cid := cidTool.GetCid()
*/

const (
	CID_BASE_COIN_SPLIT_OKX   = '9'
	CID_BASE_COIN_SPLIT_BYBIT = "_"
)

type CidTool struct {
	cidLock        sync.Mutex
	suffixCid      int                     // 用来避免同时产生大量 cid 重复问题 耗时很小
	prefixCid      string                  // 常规前缀 耗时较大
	brokerStr      string                  // 一般来说是固定的 broker string
	prefixCidFunc  func() string           // 和 pair 无关的前缀生成器
	prefixPairFunc func(pair *Pair) string // 和 pair 有关的前缀生成器
}

func NewCidTool(exName, brokerStr string) *CidTool {
	// 自动填充 binance um
	if exName == BrokernameBinanceUsdtSwap.String() || exName == BrokernameBinanceUsdtSwapUm.String() ||
		exName == BrokernameBinanceUsdSwap.String() || exName == BrokernameBinanceSpot.String() {
		if brokerStr == "" {
			brokerStr = "x-nXtHr5jj"
		}
	}
	//
	c := &CidTool{
		suffixCid: 100,
		brokerStr: brokerStr,
		// 获得对应的cid产生函数
		prefixCidFunc:  GetPrefixCidFunc(exName),
		prefixPairFunc: GetPrefixPairFunc(exName),
	}
	// 产生初始cid前缀
	c.prefixCid = c.prefixCidFunc()
	return c
}

func (b *CidTool) GetCid(pair *Pair) string {
	b.cidLock.Lock()
	defer b.cidLock.Unlock()
	b.suffixCid += 1
	if b.suffixCid > 999 {
		b.suffixCid = 100
		b.prefixCid = b.prefixCidFunc()
	}
	return b.brokerStr + b.prefixPairFunc(pair) + b.prefixCid + strconv.Itoa(b.suffixCid)
}

// 获取带字母的cid前缀
func getBaseCid() string {
	// return GetRandLetter() + GetRandLetter() + GetRandLetter() + strconv.Itoa(rand.Intn(999999))
	return "X" + GetRandLetter() + GetRandLetter() + strconv.Itoa(rand.Intn(999999)) // 第一个字母不可用M，monitor占用
}

// 获取huobi cid前缀 1-9223372036854775807
func getBaseCidForHuobi() string {
	return "1" + strconv.Itoa(rand.Intn(99999999))
}

func getBaseCidForGate() string {
	return "t-" + getBaseCid()
}

// 获取 cid前缀 全数字
func getBaseCidPureNum() string {
	return strconv.Itoa(rand.Intn(99999999))
}

// 获取 cid前缀 空
func getBaseCidEmpty() string {
	return ""
}

// 获取32位16进制字符串，不足位数前面补0，并以"0x"开头
func GetBaseCidHex() string {
	return fmt.Sprintf("0x%029x", rand.Intn(99999999))
}

// bybit 支持 ._
// 请务必注意此处的解析方式 和 bybit_usdt_swap  rs.go 中的保持一致
func getPrefixWithCoin(pair *Pair) string {
	return fmt.Sprintf("%s_", pair.Base)
}

// 字母（区分大小写）与数字的组合，可以是纯字母、纯数字且长度要在1-32位之间。
func getPrefixWithCoinForOkx(pair *Pair) string {
	return fmt.Sprintf("%s%c", pair.Base, CID_BASE_COIN_SPLIT_OKX)
}

// 根据交易所名称获取cid前缀
func GetPrefixCidFunc(ex string) func() string {
	switch ex {
	case BrokernameHuobiUsdtSwapIso.String(), BrokernameHuobiUsdtSwapUm.String(), BrokernameHuobiUsdtSwapUm2.String(),
		BrokernameHuobiUsdtSwap.String(), BrokernameBingxUsdtSwap.String(): //bingX 交易所有bug,  大写CID会变成小写传回, 先只用数字
		return getBaseCidForHuobi
	case BrokernameGateSpot.String(), BrokernameGateUsdtSwap.String(), BrokernameGateUsdtSwapSpec.String():
		return getBaseCidForGate
	case BrokernameWooUsdtSwap.String():
		return getBaseCidPureNum
	case BrokernameApexUsdtSwap.String():
		return getBaseCidPureNum
	case BrokernameVertexUsdcSwap.String():
		return getBaseCidPureNum
	case BrokernameHyperSpot.String():
		return GetBaseCidHex
	case BrokernameHyperUsdcSwap.String():
		return GetBaseCidHex
	default:
		return getBaseCid
	}
}

// 根据交易所名称获取cid前缀
func GetPrefixPairFunc(ex string) func(pair *Pair) string {
	switch ex {
	case BrokernameBybitUsdtSwap.String(), BrokernameBybitSpot.String(), BrokernameBinanceSpot.String(), BrokernameBinanceUsdtSwap.String():
		return getPrefixWithCoin
	case BrokernameOkxUsdtSwap.String(), BrokernameOkxSpot.String():
		return getPrefixWithCoinForOkx
	default:
		return func(pair *Pair) string {
			return ""
		}
	}
}
