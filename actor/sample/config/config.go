//// 策略的配置文件
//
//// 用于本策略的配置文件
package config

//
//import (
//	"encoding/base64"
//	"github.com/BurntSushi/toml"
//	"github.com/forgoer/openssl"
//	"os"
//)
//
//// 配置文件 和策略相关的配置内容
//type Config struct {
//	TargetIP    string `toml:"target_ip"`    // 交易服务器ip
//	TradeIP     string `toml:"trade_ip"`     // 下单交易的ip
//	AcctName string `toml:"account_name"` // 账户名字
//	LogLevel    string `toml:"log_level"`    // 日志等级
//	BrokerID    string `toml:"broker_id"`    // 经纪商id
//	AccessKey   string `toml:"access_key"`   // apikey
//	SecretKey   string `toml:"secret_key"`   // seckey
//	PassKey     string `toml:"pass_key"`     // paskey
//	Proxy       string `toml:"proxy"`        // 代理
//	ServerPort  int64  `toml:"server_port"`  // web server端口号
//}
//
//var (
//	Conf Config
//	// configFile string
//)
//
//// 获取配置文件的指针
//func Session() *Config {
//	return &Conf
//}
//
//// 加载配置文件
//func LoadConfig(fileName string, isEncrypt bool) *Config {
//	fPath := fileName
//	//fPath := filepath.Join(".", fileName)
//
//	// 文件是否为加密后的
//	if isEncrypt {
//		// 配置文件的密钥
//		const aesKey = "123456"
//		encryptConfig, err := os.ReadFile(fPath)
//		if err != nil {
//			panic(err)
//		}
//		encrypted, err := base64.StdEncoding.DecodeString(string(encryptConfig))
//		if err != nil {
//			panic(err)
//		}
//		newplaintext, err := openssl.AesECBDecrypt(encrypted, []byte(aesKey), openssl.PKCS7_PADDING)
//		if err != nil {
//			panic(err)
//		}
//		if _, err := toml.Decode(string(newplaintext), &Conf); err != nil {
//			panic(err)
//		}
//	} else {
//		if _, err := toml.DecodeFile(fPath, &Conf); err != nil {
//			panic(err)
//		}
//	}
//	return &Conf
//}
