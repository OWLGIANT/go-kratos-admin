package local

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"actor/broker/brokerconfig"
	"actor/helper"
	"actor/helper/helper_ding"
	"actor/third/log"
)

// 外部参考行情来源 此处要填写私有ip地址 只有专线联通的情况下才能用私有ip访问到跨区域的服务器
// 1号专线VPC
const SourceJP_1 = "172.17.6.53,172.17.7.159"                                                                        // 1号aws bm-jp-xx
const SourceJP_1_PH = "10.100.100.4:9000,10.100.100.4:9005,10.100.100.4:9010,10.100.100.4:9015,10.100.100.4:9020," + //
	"10.100.100.4:9025,10.100.100.4:9030,10.100.100.4:9035,10.100.100.4:9040,10.100.100.4:9045," + //
																"10.100.100.4:9050,10.100.100.4:9055,10.100.100.4:9060,10.100.100.4:9065,10.100.100.4:9070"
const SourceSG_1 = "172.20.12.66,172.20.13.174,172.20.13.191,172.20.6.88,172.20.13.232,172.20.6.184,172.20.13.75,172.20.14.253" // 3号aws

// 2号专线VPC
// const SourceJP_2 = "172.17.4.190:10090,172.17.15.217"
// const SourceSG = "172.19.105.132,172.19.42.87:10090"
const SourceJP_2 = "172.17.4.190"   // 1号 darkk-a-002
const SourceSG_2 = "172.19.105.132" // 1号aws

const SourceHK_1 = "192.168.42.183"

// 2号专线VPC
const SourceHK_2 = "192.168.42.118"

const SourceIE = ""
const SourceKR = "172.14.0.225"

var SourceJP string
var SourceHK string
var SourceSG string

func init() {
	SourceJP = fmt.Sprintf("%s,%s,%s", SourceJP_1, SourceJP_1_PH, SourceJP_2)
	SourceHK = fmt.Sprintf("%s,%s", SourceHK_1, SourceHK_2)
	SourceSG = fmt.Sprintf("%s,%s", SourceSG_1, SourceSG_2)
}

// 位置地址
const LocalHK = "HK"
const LocalJP = "JP"
const LocalSG = "SG"
const LocalIE = "IE"
const LocalKR = "KR"
const LocalAURORA = "AURORA"
const LocalCARTERET = "CARTERET"

// used for beast_ared.json
type Area struct {
	Area string `json:"area"`
}

// GetLocal 获取本机器位置 避免同区域走vpc
func GetLocal() string {
	var local string
	mi := brokerconfig.LoadMachineInfo()
	if mi == nil || mi.Zone == "" {
		panic(fmt.Sprintf("failed to get server area. GetLocal. zone is empty %v", mi))
	}
	areaPrefixMap := map[string]string{
		"qq-sg":            LocalSG,
		"aws-ap-northeast": LocalJP,
		"aws-ap-southeast": LocalSG,
		"ali-cn-hongkong":  LocalHK,
	}
	for prefix, localT := range areaPrefixMap {
		if strings.HasPrefix(mi.Zone, prefix) {
			return localT
		}
	}
	panic(fmt.Sprintf("unknown zone %v", mi))

	// 获取本机位置
	client := http.Client{
		Timeout: 5 * time.Second, // 设置超时时间
	}
	r, e := client.Get("http://ip-api.com/json")
	if e != nil {

	} else {
		if r.StatusCode == 200 {
			var location string
			body, _ := io.ReadAll(r.Body)
			msg := helper.BytesToString(body)
			log.Infof("当前服务器位置msg:%v", msg)
			var m map[string]string
			json.Unmarshal(body, &m)
			city := m["city"]
			countryCode := m["countryCode"]
			if strings.Contains(city, "Hong Kong") {
				location = LocalHK
			} else if strings.Contains(city, "Singapore") {
				location = LocalSG
			} else if strings.Contains(city, "Tokyo") {
				location = LocalJP
			} else if strings.Contains(countryCode, "IE") {
				location = LocalIE
			} else if strings.Contains(countryCode, "KR") {
				location = LocalKR
			} else if strings.Contains(countryCode, "HK") {
				location = LocalHK
			} else if strings.Contains(countryCode, "SG") {
				location = LocalSG
			}
			log.Infof("当前服务器位置:%v", location)
			local = location
			if local == "" {
				helper_ding.DingingSendSerious(fmt.Sprintf("failed to get server area. GetLocal. %s", msg))
			}
		} else {
			helper_ding.DingingSendSerious(fmt.Sprintf("failed to get server area. GetLocal. %d", r.StatusCode))
		}
		defer r.Body.Close()
	}
	return local
}

// GetExchangeLocal 获取交易所的服务器位置
func GetExchangeLocal(refName string) string {
	switch refName {
	// JP
	case helper.BrokernameBinanceUsdtSwap.String():
		return LocalJP
	case helper.BrokernameBinanceUsdSwap.String():
		return LocalJP
	case helper.BrokernameBinanceSpot.String():
		return LocalJP
	case helper.BrokernameGateSpot.String():
		return LocalJP
	case helper.BrokernameGateUsdtSwap.String():
		return LocalJP
	case helper.BrokernameCoinexSpot.String():
		return LocalJP
	case helper.BrokernameCoinexUsdtSwap.String():
		return LocalJP
	case helper.BrokernameKucoinSpot.String():
		return LocalJP
	case helper.BrokernameKucoinUsdtSwap.String():
		return LocalJP
	case helper.BrokernameBitgetUsdtSwap.String():
		return LocalJP
	case helper.BrokernameBitgetUsdtSwapV2.String():
		return LocalJP
	case helper.BrokernameHuobiUsdtSwap.String():
		return LocalJP
	case helper.BrokernameHuobiSpot.String():
		return LocalJP
	case helper.BrokernameWooUsdtSwap.String():
		return LocalJP // gcp tokyo
	case helper.BrokernameBitmartUsdtSwap.String():
		return LocalJP
	case helper.BrokernameBitmartSpot.String():
		return LocalJP

	// HK
	case helper.BrokernameOkxSpot.String():
		return LocalHK
	case helper.BrokernameOkxUsdtSwap.String():
		return LocalHK

	// SG
	case helper.BrokernameBybitUsdtSwap.String():
		return LocalSG
	case helper.BrokernamePhemexUsdtSwap.String():
		return LocalSG
	// USA
	case helper.BrokernameCME.String():
		return LocalAURORA
	case helper.BrokernameNASDAQ.String():
		return LocalCARTERET
	// Other
	case helper.BrokernameBitmexUsdtSwap.String():
		return LocalIE
	case helper.BrokernameUpbitSpot.String():
		return LocalKR
	default:
		return "Unknown Location"
	}
}

// MarketServer 连本区节点。节点会包含本区普通链接和跨域链接
// param exName like binance_usdt_swap
func GetMarketServers4Ex(exName string) ([]string, error) {
	exName = strings.TrimSpace(exName)
	exArea := GetExchangeLocal(exName)
	if exArea == "Unknown Location" {
		return nil, errors.New("Unknown Exchange")
	}
	local := GetLocal()
	if local == exArea {
		return nil, errors.New("Same Area, should not use cross market")
	}
	switch local {
	case LocalHK:
		return strings.Split(SourceHK, ","), nil
	case LocalSG:
		return strings.Split(SourceSG, ","), nil
	case LocalJP:
		return strings.Split(SourceJP, ","), nil
	case LocalIE:
		return strings.Split(SourceIE, ","), nil
	case LocalKR:
		return strings.Split(SourceKR, ","), nil
	default:
		return nil, errors.New("Unknown Local")
	}
}
