// 策略的配置文件
// 用于本策略的配置文件
package config

import (
	"actor/helper"
	"encoding/json"
	"fmt"
	"os"
)

// Config 启动配置文件
type Config struct {
	/* 必传参数 */
	TaskUid  string `json:"task_uid"`  // 任务ID 同后端的机器人uid
	TradeIP  string `json:"trade_ip"`  // 下单交易的ip 如果是单ip服务器请留空 如果多ip服务器 留空默认主ip 否则查找对应的ip发送http
	BrokerID string `json:"broker_id"` // 经纪商id 一般留空 来自后端添加账户时填写的内容
	/* 账户参数 */
	AcctName  string `json:"account_name"` // 账户名字 来自后端添加账户时填写的内容
	AccessKey string `json:"access_key"`   // api_key 来自后端添加账户时填写的内容
	SecretKey string `json:"secret_key"`   // secret_key 来自后端添加账户时填写的内容
	PassKey   string `json:"pass_key"`     // pass_key 来自后端添加账户时填写的内容
	Exchange  string `json:"exchange"`     // exchange 来自后端添加账户时填写的内容
	/* 系统参数 不需要外部传入 读取本地配置文件 */
	Proxy      string `json:"proxy"`       // 代理
	LogLevel   string `json:"log_level"`   // 日志等级
	Port       int    `json:"port"`        // 端口号
	ServerPort int    `json:"server_port"` // 托管者端口号

	/* 策略参数 */
	Strategy       string         `json:"strategy"`        // 策略名字 来策略模版
	StrategyParams StrategyConfig `json:"strategy_params"` // 策略模版参数 来策略模版
}

type StrategyConfig struct {
	Pair     string  `json:"pair"`      // 交易品种
	Lever    float64 `json:"lever"`     // 杠杆
	Windows  int     `json:"windows"`   //行情窗口
	StopLoss float64 `json:"stop_loss"` // 止损点
	WinLess  float64 `json:"win_less"`  // 最少盈利点
}

var (
	Conf Config
)

// Session 获取配置文件的指针
func Session() *Config {
	fmt.Println("传入参数", Conf.StrategyParams.StopLoss)
	return &Conf
}

// LoadConfig 加载配置文件
func LoadConfig(fileName string, isEncrypt bool) *Config {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	err = decoder.Decode(&Conf)
	if err != nil {
		fmt.Println(err)
	}
	if isEncrypt {
		Conf.SecretKey = helper.AesDecrypt(Conf.SecretKey, "l#Os#*JFJX!0^yOI")
	}
	return &Conf
}

// LoadConfigFromMap 从 map 加载配置 (用于从 backend 接收配置)
func LoadConfigFromMap(data map[string]interface{}) *Config {
	Conf = Config{}

	if v, ok := data["task_uid"].(string); ok {
		Conf.TaskUid = v
	}
	if v, ok := data["trade_ip"].(string); ok {
		Conf.TradeIP = v
	}
	if v, ok := data["broker_id"].(string); ok {
		Conf.BrokerID = v
	}
	if v, ok := data["account_name"].(string); ok {
		Conf.AcctName = v
	}
	if v, ok := data["access_key"].(string); ok {
		Conf.AccessKey = v
	}
	if v, ok := data["secret_key"].(string); ok {
		Conf.SecretKey = v
	}
	if v, ok := data["pass_key"].(string); ok {
		Conf.PassKey = v
	}
	if v, ok := data["exchange"].(string); ok {
		Conf.Exchange = v
	}
	if v, ok := data["proxy"].(string); ok {
		Conf.Proxy = v
	}
	if v, ok := data["log_level"].(string); ok {
		Conf.LogLevel = v
	}
	if v, ok := data["port"].(float64); ok {
		Conf.Port = int(v)
	}
	if v, ok := data["server_port"].(float64); ok {
		Conf.ServerPort = int(v)
	}
	if v, ok := data["strategy"].(string); ok {
		Conf.Strategy = v
	}

	// 解析策略参数
	if params, ok := data["strategy_params"].(map[string]interface{}); ok {
		if v, ok := params["pair"].(string); ok {
			Conf.StrategyParams.Pair = v
		}
		if v, ok := params["lever"].(float64); ok {
			Conf.StrategyParams.Lever = v
		}
		if v, ok := params["windows"].(float64); ok {
			Conf.StrategyParams.Windows = int(v)
		}
		if v, ok := params["stop_loss"].(float64); ok {
			Conf.StrategyParams.StopLoss = v
		}
		if v, ok := params["win_less"].(float64); ok {
			Conf.StrategyParams.WinLess = v
		}
	}

	return &Conf
}
