// 录制
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "net/http/pprof"

	"actor"
	"actor/broker/base"
	"actor/helper"
	"actor/third/log"
	"github.com/BurntSushi/toml"
)

var logFile string

func main() {

	flag.StringVar(&logFile, "l", "log.log", "set log `file`")
	var ex = flag.String("ex", "", "like binance_usdt_swap")
	var pairStr = flag.String("pair", "", "like btc_usdt")
	var keyFile = flag.String("key_file", "", "")
	flag.Parse()

	log.Init(logFile, "debug",
		// log.SetStdout(true),
		log.SetCaller(true),
		log.SetMaxBackups(1),
	)

	go func() {
		fmt.Println("pprof start...")
		fmt.Println(http.ListenAndServe(":80", nil))
	}()

	pair, err := helper.StringPairToPair(*pairStr)
	if err != nil {
		panic(err)
	}

	var cfg interface{}
	var firstFileContent []byte
	firstFileContent, err = os.ReadFile(*keyFile)
	if err != nil {
		panic(err)
	}

	if err := toml.Unmarshal(firstFileContent, &cfg); err != nil {
		panic(err)
	}

	cfgMap := cfg.(map[string]interface{})
	access_key := cfgMap["access_key"].(string)
	secret_key := cfgMap["secret_key"].(string)
	pass_key := cfgMap["pass_key"].(string)

	refRsCb := helper.CallbackFunc{
		OnTicker:   func(ts int64) {},
		OnEquity:   func(ts int64) {},
		OnPosition: func(ts int64) {},
		OnOrder:    func(ts int64, event helper.OrderEvent) {},
		OnDepth:    func(ts int64) {},
		OnReset:    func(nsg string) {},
		OnExit:     func(nsg string) {},
	}
	rsTradeMsg := helper.TradeMsg{}
	rsPairInfo := helper.ExchangeInfo{}
	rs := actor.GetRsClient(*ex, helper.BrokerConfig{
		Name:      helper.BrokernameBinanceUsdSwap.String(),
		AccessKey: access_key,
		SecretKey: secret_key,
		PassKey:   pass_key,
		Pair:      pair,
		ProxyURL:  "",
		NeedAuth:  true,
		LocalAddr: "",
	},
		&rsTradeMsg,
		&rsPairInfo,
		refRsCb)
	if rs == nil {
		panic("failed to create rs client. ")
	}
	rsep, ok := rs.(base.RsExposer)
	if !ok {
		panic("rs not implemented RsExposer")
	}
	rsep.GetExchangeInfos() // for set symbol

	wsTradeMsg := helper.TradeMsg{}
	wsPairInfo := helper.ExchangeInfo{}
	ws := actor.GetWsClient(*ex, helper.BrokerConfig{
		Name:       helper.BrokernameBinanceUsdSwap.String(),
		AccessKey:  access_key,
		SecretKey:  secret_key,
		PassKey:    pass_key,
		Pair:       pair,
		ProxyURL:   "",
		NeedAuth:   true,
		NeedTicker: true,
		LocalAddr:  "",
	},
		&wsTradeMsg,
		&wsPairInfo,
		refRsCb)
	ws.Run()

	for {
		// time.Sleep(time.Hour * 24 * 365)
		time.Sleep(time.Second * 10)
		rs.SendSignal([]helper.Signal{{
			Type: helper.SignalTypeGetEquity,
		}})
	}
}
