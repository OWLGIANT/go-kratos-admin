// 用于本策略的配置文件
package brokerconfig

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"actor/config"
	"actor/helper"
	"actor/third/log"
	"go.uber.org/atomic"

	"github.com/BurntSushi/toml"
)

// 配置文件 和策略相关的配置内容
type HostConfig struct {
	// 币安现货
	BinanceSpotWsUrl   string `toml:"binance_spot__ws_url"`
	BinanceSpotRestUrl string `toml:"binance_spot__rest_url"`
	//BinanceSpotProxy   string `toml:"binance_spot__proxy"`
	// 币安U本位合约
	BinanceUsdtSwapWsUrl   string `toml:"binance_usdt_swap__ws_url"`
	BinanceUsdtSwapRestUrl string `toml:"binance_usdt_swap__rest_url"`
	// 币安币本位合约
	BinanceUsdSwapWsUrl   string `toml:"binance_usd_swap__ws_url"`
	BinanceUsdSwapRestUrl string `toml:"binance_usd_swap__rest_url"`
	//BinanceUsdtSwapProxy   string `toml:"binance_usdt_swap__proxy"`
	// 火币现货
	HuobiSpotWsUrl   string `toml:"huobi_spot__ws_url"`
	HuobiSpotRestUrl string `toml:"huobi_spot__rest_url"`
	// superex现货
	SuperexSpotWsUrl   string `toml:"superex_spot__ws_url"`
	SuperexSpotRestUrl string `toml:"superex_spot__rest_url"`
	// 火币合约
	HuobiUsdtSwapWsUrl   string `toml:"huobi_usdt_swap__ws_url"`
	HuobiUsdtSwapRestUrl string `toml:"huobi_usdt_swap__rest_url"`
	// gate现货
	GateSpotWsUrl   string `toml:"gate_spot__ws_url"`
	GateSpotRestUrl string `toml:"gate_spot__rest_url"`
	// gate合约
	GateUsdtSwapWsUrl   string `toml:"gate_usdt_swap__ws_url"`
	GateUsdtSwapRestUrl string `toml:"gate_usdt_swap__rest_url"`
	// phemex现货
	PhemexSpotWsUrl   string `toml:"phemex_spot__ws_url"`
	PhemexSpotRestUrl string `toml:"phemex_spot__rest_url"`
	// okx合约
	OkxUsdtSwapWsPubUrl string `toml:"okx_usdt_swap__ws_pub_url"`
	OkxUsdtSwapWsPriUrl string `toml:"okx_usdt_swap__ws_pri_url"`
	OkxUsdtSwapRestUrl  string `toml:"okx_usdt_swap__rest_url"`
	OkxUsdtSwapProxy    string `toml:"okx_usdt_swap__proxy"`
	// okx现货
	OkxUsdtSpotWsPubUrl string `toml:"okx_usdt_spot__ws_pub_url"`
	OkxUsdtSpotWsPriUrl string `toml:"okx_usdt_spot__ws_pri_url"`
	OkxUsdtSpotRestUrl  string `toml:"okx_usdt_spot__rest_url"`
	OkxUsdtSpotProxy    string `toml:"okx_usdt_spot__proxy"`
	// bybit合约
	BybitUsdtSwapWsUrl   string `toml:"bybit_usdt_swap__ws_url"`
	BybitUsdtSwapRestUrl string `toml:"bybit_usdt_swap__rest_url"`
	// bybit现货
	BybitSpotWsUrl   string `toml:"bybit_spot__ws_url"`
	BybitSpotRestUrl string `toml:"bybit_spot__rest_url"`
	// bitmex合约
	BitmexUsdtSwapWsPubUrl string `toml:"bitmex_usdt_swap__ws_pub_url"`
	BitmexUsdtSwapWsPriUrl string `toml:"bitmex_usdt_swap__ws_pri_url"`
	BitmexUsdtSwapRestUrl  string `toml:"bitmex_usdt_swap__rest_url"`
	//bitmex USD合约
	BitmexInvSwapWsPubUrl string `toml:"bitmex_inv_swap__ws_pub_url"`
	BitmexInvSwapWsPriUrl string `toml:"bitmex_inv_swap__ws_pri_url"`
	BitmexInvSwapRestUrl  string `toml:"bitmex_inv_swap__rest_url"`
	//Phemex合约
	PhemexUsdtSwapWsPubUrl string `toml:"phemex_usd_swap__ws_pub_url"`
	PhemexUsdtSwapWsPriUrl string `toml:"phemex_usd_swap__ws_pri_url"`
	PhemexUsdtSwapRestUrl  string `toml:"phemex_usd_swap__rest_url"`
	//bitmart合约
	BitmartUsdtSwapWsUrl   string `toml:"bitmart_usdt_swap__ws_url"`
	BitmartUsdtSwapRestUrl string `toml:"bitmart_usdt_swap__rest_url"`
	// bitmart现货
	BitmartSpotWsUrl   string `toml:"bitmart_spot__ws_url"`
	BitmartSpotRestUrl string `toml:"bitmart_spot__rest_url"`
	// ascendex合约
	AscendexUsdtSwapWsUrl   string `toml:"ascendex_usdt_swap__ws_url"`
	AscendexUsdtSwapRestUrl string `toml:"ascendex_usdt_swap__rest_url"`
	BitmexUsdSwapWsPubUrl   string `toml:"bitmex_usd_swap__ws_pub_url"`
	BitmexUsdSwapWsPriUrl   string `toml:"bitmex_usd_swap__ws_pri_url"`
	BitmexUsdSwapRestUrl    string `toml:"bitmex_usd_swap__rest_url"`
	// bitfinex 合约
	BitfinexUsdtSwapRestUrl string `toml:"bitfinex_usdt_swap__rest_url"`

	LbankUsdtSwapWsUrl   string `toml:"lbank_usdt_swap__ws_url"`
	LbankUsdtSwapRestUrl string `toml:"lbank_usdt_swap__rest_url"`

	CoinbaseUsdcSwapWsUrl   string `toml:"coinbase_usdc_swap__ws_url"`
	CoinbaseUsdcSwapRestUrl string `toml:"coinbase_usdc_swap__rest_url"`
	CoinbaseSpotWsUrl       string `toml:"coinbase_spot__ws_url"`
	CoinbaseSpotRestUrl     string `toml:"coinbase_spot__rest_url"`

	// woo合约
	WooUsdtSwapWsUrl   string `toml:"woo_usdt_swap__ws_url"`
	WooUsdtSwapRestUrl string `toml:"woo_usdt_swap__rest_url"`

	ApexUsdtSwapWsUrl   string `toml:"apex_usdt_swap__ws_url"`
	ApexUsdtSwapRestUrl string `toml:"apex_usdt_swap__rest_url"`

	//合约
	DydxUsdcSwapWsUrl   string `toml:"dydx_usdc_swap__ws_url"`
	DydxUsdcSwapRestUrl string `toml:"dydx_usdc_swap__rest_url"`

	AevoUsdcSwapWsUrl   string `toml:"aevo_usdc_swap__ws_url"`
	AevoUsdcSwapRestUrl string `toml:"aevo_usdc_swap__rest_url"`

	VertexUsdcSwapWsUrl   string `toml:"vertex_usdc_swap__ws_url"`
	VertexUsdcSwapRestUrl string `toml:"vertex_usdc_swap__rest_url"`

	MexcSpotRsUrl string `toml:"mexc_spot__rs_url"`
	MexcSpotWsUrl string `toml:"mexc_spot__ws_url"`

	MexcUsdtSwapWsUrl   string `toml:"mexc_usdt_swap__ws_url"`
	MexcUsdtSwapRestUrl string `toml:"mexc_usdt_swap__rest_url"`

	// hashkey现货
	HashkeySpotWsUrl   string `toml:"hashkey_spot__ws_url"`
	HashkeySpotRestUrl string `toml:"hashkey_spot__rest_url"`

	BitgetUsdtSwapWsUrl    string `toml:"bitget_usdt_swap__ws_url"`
	BitgetUsdtSwapPubWsUrl string `toml:"bitget_usdt_swap__pub_ws_url"`
	BitgetUsdtSwapRestUrl  string `toml:"bitget_usdt_swap__rest_url"`
	BitgetSpotWsUrl        string `toml:"bitget_spot__ws_url"`
	BitgetSpotPubWsUrl     string `toml:"bitget_spot__pub_ws_url"`
	BitgetSpotRestUrl      string `toml:"bitget_spot__rest_url"`

	// HyperLiquid
	HyperSpotWsUrl       string `toml:"hyperliquid_spot__ws_url"`
	HyperSpotRestUrl     string `toml:"hyperliquid_spot__rest_url"`
	HyperUsdtSwapWsUrl   string `toml:"hyperliquid_usdt_swap__ws_url"`
	HyperUsdtSwapRestUrl string `toml:"hyperliquid_usdt_swap__rest_url"`

	BitSpotWsUrl       string `toml:"bit_spot__ws_url"`
	BitSpotRestUrl     string `toml:"bit_spot__rest_url"`
	BitUsdtSwapWsUrl   string `toml:"bit_usdt_swap__ws_url"`
	BitUsdtSwapRestUrl string `toml:"bit_usdt_swap__rest_url"`
}

var Center = map[string][2]string{ // val:[config_center, data_center]
	"phy": {"172.20.14.253,172.21.2.133", "172.20.14.253,172.21.6.150"}, // proxy, target
}

// 循环依赖，所以放这里
func initSystemConfig() {
	machineInfo := LoadMachineInfo()
	var redisAddr, redisPwd string
	var influxAddr, influxPwd, influxUser string
	if strings.HasPrefix(machineInfo.Zone, "ali") {
		// 网段覆盖，先不用

	} else if strings.HasPrefix(machineInfo.Zone, "aws") {
		//switch machineInfo.AwsAcct {
		//case "phy":
		//	redisAddr = fmt.Sprintf("%s:6379", Center[machineInfo.AwsAcct][0])
		//	redisPwd = "laIKsuejs8392NKS"
		//	influxAddr = fmt.Sprintf("%s:8086", Center[machineInfo.AwsAcct][1])
		//	influxUser = "root"
		//	influxPwd = "jLtV46r9WkYitafB"
		//default:
		//	msg := "not support aws acct. " + machineInfo.AwsAcct
		//	fmt.Println(msg)
		//	panic(msg)
		//}
	}
	config.Set(redisAddr, redisPwd, influxAddr, influxUser, influxPwd)
}

// 获取中心地址, [config_center, data_center]
func GetCenter() (string, string) {
	succ := false
	defer func() {
		if !succ {
			if err := recover(); err != nil {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				fmt.Fprintf(os.Stderr, "==> %s\n", string(buf[:n]))
			}
		}
	}()
	machineInfo := LoadMachineInfo()
	vals, ok := Center[machineInfo.AwsAcct]
	if !ok {
		panic("not support aws acct. " + machineInfo.AwsAcct)
	}
	succ = true
	return vals[0], vals[1]
}

func init() {
	initSystemConfig()
}

var (
	HostConf HostConfig
	// configFile string
)

func BrokerSession() *HostConfig {
	return &HostConf
}

type MachineInfo_T struct {
	inited                atomic.Bool
	Zone                  string `toml:"zone"`                    // 可用区, aws-ap-northeast-1a, aws/ali 开头
	AwsAcct               string `toml:"aws_acct"`                //
	BgColo                bool   `toml:"bg_colo"`                 //是否bg vip 服务器
	IgnoreIpCheck         bool   `toml:"ignore_ip_check"`         // 是否忽略ip检查
	StorageAlertThreshold int    `toml:"storage_alert_threshold"` // 存储告警阈值，单位GB
}

var MachineInfo MachineInfo_T

// GetColoStatus 检查是否启用了colo
// 情况1 通过修改url启动colo 检查 beast.toml
// 情况2 通过修改hosts启用colo 检查 /etc/hosts
func GetColoStatus(exchangeName string) bool {
	switch exchangeName {
	case helper.BrokernameGateSpot.String():
		if HostConf.GateSpotRestUrl != "" && HostConf.GateSpotWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameGateUsdtSwap.String():
		if HostConf.GateUsdtSwapRestUrl != "" && HostConf.GateUsdtSwapWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameBinanceUsdtSwap.String():
		if HostConf.BinanceUsdtSwapRestUrl != "" && HostConf.BinanceUsdtSwapWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameHuobiUsdtSwap.String():
		if HostConf.HuobiUsdtSwapRestUrl != "" && HostConf.HuobiUsdtSwapWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameHuobiSpot.String():
		if HostConf.HuobiSpotRestUrl != "" && HostConf.HuobiSpotWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameAscendexUsdtSwap.String():
		if HostConf.AscendexUsdtSwapRestUrl != "" && HostConf.AscendexUsdtSwapWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameBybitUsdtSwap.String():
		if HostConf.BybitUsdtSwapRestUrl != "" && HostConf.BybitUsdtSwapWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameBybitSpot.String():
		if HostConf.BybitSpotRestUrl != "" && HostConf.BybitSpotWsUrl != "" {
			return true
		} else {
			return false
		}
	case helper.BrokernameKucoinUsdtSwap.String():
		cmd := exec.Command("cat", "/etc/hosts")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		if strings.Contains(helper.BytesToString(output), "kucoin") {
			return true
		}
		return false
	case helper.BrokernameKucoinSpot.String():
		cmd := exec.Command("cat", "/etc/hosts")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		if strings.Contains(helper.BytesToString(output), "kucoin") {
			return true
		}
		return false
	case helper.BrokernameOkxUsdtSwap.String():
		return HostConf.OkxUsdtSwapRestUrl != "" && HostConf.OkxUsdtSwapWsPriUrl != "" && HostConf.OkxUsdtSwapWsPubUrl != ""
	case helper.BrokernameOkxSpot.String():
		return HostConf.OkxUsdtSpotRestUrl != "" && HostConf.OkxUsdtSpotWsPriUrl != "" && HostConf.OkxUsdtSpotWsPubUrl != ""
	case helper.BrokernameBitgetSpot.String():
		return HostConf.BitgetSpotRestUrl != "" && HostConf.BitgetSpotWsUrl != ""
	case helper.BrokernameBitgetUsdtSwap.String():
		return HostConf.BitgetSpotRestUrl != "" && HostConf.BitgetSpotWsUrl != ""
	case helper.BrokernameBitgetSpotV2.String():
		return HostConf.BitgetSpotRestUrl != "" && HostConf.BitgetSpotWsUrl != ""
	case helper.BrokernameBitgetUsdtSwapV2.String():
		return HostConf.BitgetSpotRestUrl != "" && HostConf.BitgetSpotWsUrl != ""
	case helper.BrokernameBitgetSpotUm.String():
		return HostConf.BitgetUsdtSwapRestUrl != "" && HostConf.BitgetUsdtSwapWsUrl != ""
	default:
		wsUrlKey := fmt.Sprintf("%s__ws_url", strings.Split(exchangeName, ".")[0])
		restUrlKey := fmt.Sprintf("%s__rest_url", strings.Split(exchangeName, ".")[0])

		cmd := exec.Command("cat", "/root/supervisor/beast.toml")
		output, err := cmd.Output()
		if err != nil {
			log.Warnf("读取beast.toml失败: %v", err)
			return false
		}

		content := helper.BytesToString(output)
		lines := strings.Split(content, "\n")

		wsUrlValue := ""
		restUrlValue := ""

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue // 跳过空行和注释
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")

			if key == wsUrlKey {
				wsUrlValue = value
			} else if key == restUrlKey {
				restUrlValue = value
			}
		}

		return wsUrlValue != "" && restUrlValue != ""
	}
}

func LoadMachineInfo() *MachineInfo_T {
	if MachineInfo.inited.Load() {
		return &MachineInfo
	}

	MachineInfo.IgnoreIpCheck = true

	MachineInfo.inited.Store(true)
	return &MachineInfo
}

func LoadBaseConfig(fileName string) *HostConfig {
	_, err := os.Stat(fileName)
	if err != nil {
		// 文件不存在
		// baseconfig文件不存在，则不解析，不需要panic
		return nil
	}

	if _, err := toml.DecodeFile(fileName, &HostConf); err != nil {
		panic(err)
	}
	return &HostConf
}
