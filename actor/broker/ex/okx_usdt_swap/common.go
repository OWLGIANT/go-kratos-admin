// okx库通用文件

package okx_usdt_swap

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"actor/broker/base"
	"actor/broker/brokerconfig"
	"actor/broker/client/rest"
	"actor/helper"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	okxWsPubUrl  = "wss://ws.okx.com:8443/ws/v5/public"
	okxWsPriUrl  = "wss://ws.okx.com:8443/ws/v5/private"
	okxWsBusUrl  = "wss://ws.okx.com:8443/ws/v5/business"
	okxRestUrl   = "https://www.okx.com"
	okxSimHeader = map[string]interface{}{}
)

var (
	wsPublicHandyPool   fastjson.ParserPool
	wsPrivateHandyPool  fastjson.ParserPool
	wsBusinessHandyPool fastjson.ParserPool
	handyPool           fastjson.ParserPool

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	GET  = http.MethodGet
	POST = http.MethodPost
)

// symbolToPair 交易所规范的symbol转换为内部数据结构 btc_usdt to BTC-USDT-SWAP
func symbolToPair(symbol string) helper.Pair {
	p := strings.Split(symbol, "-")
	if len(p) == 3 {
		p0 := helper.Pair{
			Base:  strings.ToLower(p[0]),
			Quote: strings.ToLower(p[1]),
			More:  "",
			// More:  strings.ToLower(p[2]),
		}
		return p0
	} else {
		return helper.Pair{
			Base:  "",
			Quote: "",
			More:  "",
		}
	}
}

// checkColo 检查是否要启用colo
func checkColo(params *helper.BrokerConfigExt) bool {
	flag := false
	if params.BanColo {
		return flag
	}
	cfg := brokerconfig.BrokerSession()
	if cfg.OkxUsdtSwapWsPubUrl != "" {
		okxWsPubUrl = cfg.OkxUsdtSwapWsPubUrl
		log.Infof("okx_usdt_swap ws启用colo pub %v", okxWsPubUrl)
		flag = true
	}
	if cfg.OkxUsdtSwapWsPriUrl != "" {
		okxWsPriUrl = cfg.OkxUsdtSwapWsPriUrl
		log.Infof("okx_usdt_swap ws启用colo pri %v", okxWsPriUrl)
		flag = true
	}
	if cfg.OkxUsdtSwapRestUrl != "" {
		okxRestUrl = cfg.OkxUsdtSwapRestUrl
		log.Infof("okx_usdt_swap rest启用colo %v", okxRestUrl)
		flag = true
	}
	if cfg.OkxUsdtSwapProxy != "" {
		proxies := strings.Split(cfg.OkxUsdtSwapProxy, ",")
		rand.Seed(time.Now().Unix())
		i := rand.Int31n(int32(len(proxies)))
		proxy := proxies[i]
		params.ProxyURL = proxy
		log.Infof("okx_usdt_swap ws 启用代理: %v", params.ProxyURL)
		flag = true
	}
	return flag
}

// SetSim 使用模拟盘需要事先调用
func SetSim() {
	okxWsPubUrl = "wss://wspap.okx.com:8443/ws/v5/public?brokerId=9999"
	okxWsPriUrl = "wss://wspap.okx.com:8443/ws/v5/private?brokerId=9999"
	okxSimHeader = map[string]interface{}{"x-simulated-trading": 1}
}

//
// 下面是一些求 sign 的算法

func MD5Sign(secret, params string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(params))

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func HmacSHA256Sign(secret, params string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HmacSHA512Sign(secret, params string) (string, error) {
	mac := hmac.New(sha512.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HmacSHA1Sign(secret, params string) (string, error) {
	mac := hmac.New(sha1.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HmacMD5Sign(secret, params string) (string, error) {
	mac := hmac.New(md5.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HmacSha384Sign(secret, params string) (string, error) {
	mac := hmac.New(sha512.New384, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", nil
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func HmacSHA256Base64Sign(secret, params string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(params))
	if err != nil {
		return "", err
	}
	signByte := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(signByte), nil
}

func HmacSHA512Base64Sign(hmacKey string, data string) string {
	hmh := hmac.New(sha512.New, []byte(hmacKey))
	hmh.Write([]byte(data))

	hexData := hex.EncodeToString(hmh.Sum(nil))
	hashHmacBytes := []byte(hexData)
	hmh.Reset()

	return base64.StdEncoding.EncodeToString(hashHmacBytes)
}

var timeOffset = -time.Second * 2

func SetTimeOffset(offset time.Duration) {
	timeOffset = offset
}

// GetIsoTimeString 获取ok所需的iso时间
func GetIsoTimeString(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000Z")
}

// GetUTCTime 获取utc时间
func GetUTCTime() time.Time {
	utcLocal, _ := time.LoadLocation("UTC")
	return time.Now().In(utcLocal)
}

func GetAdjustTime() time.Time {
	return GetUTCTime().Add(timeOffset)
}

func makeHeaders() map[string]string { // 只有在第一次调用的时候创建，后面都复用
	h := make(map[string]string, 0)
	for k, v := range okxSimHeader { // 处理模拟盘所需的一些头
		h[k] = fmt.Sprintf("%v", v)
	}
	h["Content-Type"] = "application/json"
	h["accept"] = "application/json"
	return h
}

// OkxUsdtSwapClient 用来执行http的操作
type OkxUsdtSwapClient struct {
	restUrl string                  // rest地址
	params  *helper.BrokerConfigExt // 配置
	pair    helper.Pair
	client  *rest.Client // 通用rest客户端
	cb      helper.CallbackFunc
	// exchangeInfoS2P map[string]helper.ExchangeInfo
	// exchangeInfoP2S map[string]helper.ExchangeInfo
}

func NewClient(params *helper.BrokerConfigExt, cb helper.CallbackFunc) *OkxUsdtSwapClient {
	return &OkxUsdtSwapClient{
		restUrl: okxRestUrl,
		params:  params,
		pair:    params.Pairs[0],
		client:  rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		cb:      cb,
		// exchangeInfoS2P: make(map[string]helper.ExchangeInfo, 0),
		// exchangeInfoP2S: make(map[string]helper.ExchangeInfo, 0),
	}
}

func (c *OkxUsdtSwapClient) get(reqUrl string, params map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler) (int, error) {
	h := makeHeaders() // todo 从函数里面return出来会多一次copy c.client.Request里面的req.header是复用的 减少了copy
	encode := make([]string, 0)
	for key, param := range params {
		encode = append(encode, fmt.Sprintf("%s=%v", key, param))
	}
	if len(encode) > 0 {
		encodes := strings.Join(encode, "&")
		reqUrl += "?" + encodes
	}
	if needSign {
		now := GetIsoTimeString(GetAdjustTime())
		h["OK-ACCESS-TIMESTAMP"] = now
		h["OK-ACCESS-KEY"] = c.params.AccessKey
		h["OK-ACCESS-PASSPHRASE"] = c.params.PassKey
		d := now + GET + reqUrl
		sign, _ := HmacSHA256Base64Sign(c.params.SecretKey, d)
		h["OK-ACCESS-SIGN"] = sign
	}
	return c.client.Request(GET, fmt.Sprintf("%s%s", c.restUrl, reqUrl), []byte{}, h, respHandler)
}

func (c *OkxUsdtSwapClient) post(reqUrl string, params map[string]interface{}, respHandler rest.FastHttpRespHandler) (int, error) {
	h := makeHeaders()
	data, _ := json.Marshal(params)
	now := GetIsoTimeString(GetAdjustTime())
	h["OK-ACCESS-TIMESTAMP"] = now
	h["OK-ACCESS-KEY"] = c.params.AccessKey
	h["OK-ACCESS-PASSPHRASE"] = c.params.PassKey
	d := now + POST + reqUrl + string(data)
	sign, _ := HmacSHA256Base64Sign(c.params.SecretKey, d)
	h["OK-ACCESS-SIGN"] = sign
	return c.client.Request(POST, fmt.Sprintf("%s%s", c.restUrl, reqUrl), data, h, respHandler)
}

func (c *OkxUsdtSwapClient) updateTime() bool {
	ok := false
	url := "/api/v5/public/time"
	params := map[string]interface{}{}
	p := handyPool.Get()
	defer handyPool.Put(p)
	ts := GetUTCTime().Unix()
	for _, i := range []int64{-2, -5, -10} {
		d, _ := time.ParseDuration(fmt.Sprintf("-%ds", i))
		SetTimeOffset(d)
		_, _ = c.get(url, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			value, handlerErr := p.ParseBytes(respBody)
			if handlerErr != nil {
				log.Errorf(fmt.Sprintf("okx getExchangeInfo http err %v", handlerErr))
				return
			}
			if code := helper.BytesToString(value.GetStringBytes("code")); code != "0" {
				return
			}
			data := value.GetArray("data")[0]
			sTs, _ := fastfloat.ParseInt64(helper.BytesToString(data.GetStringBytes("ts")))
			offset := ts - sTs/1000 - 2
			offsetStr := fmt.Sprintf("%ds", offset)
			tmp, _ := time.ParseDuration(offsetStr)
			SetTimeOffset(tmp)
			ok = true
		})
		if ok {
			break
		}
	}
	return ok
}

// func (c *OkxUsdtSwapClient) updateExchangeInfo(e *helper.ExchangeInfo) {
// 	c.getExchangeInfo(e, c.exchangeInfoS2P, c.exchangeInfoP2S)
// 	// helper.CopySymbolInfo(e, n)
// }

func (rs *OkxUsdtSwapClient) getExchangeInfo(infoIn *helper.ExchangeInfo, exchangeInfoPtrS2P, exchangeInfoPtrP2S cmap.ConcurrentMap[string, *helper.ExchangeInfo]) (*helper.ExchangeInfo, []helper.ExchangeInfo) {
	infos := make([]helper.ExchangeInfo, 0)
	// 尝试从文件中读取exchangeInfo
	fileName := base.GenExchangeInfoFileName("")
	if pairInfo, infos, ok := helper.TryGetExchangeInfosPtrFromFile(fileName, rs.pair, exchangeInfoPtrS2P, exchangeInfoPtrP2S); ok {
		helper.CopySymbolInfo(infoIn, &pairInfo)
		return infoIn, infos
	}

	url := "/api/v5/public/instruments"
	params := map[string]interface{}{"instType": "SWAP"}
	_, err := rs.get(url, params, false, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value ExchangeInfo
		handlerErr := jsoniter.Unmarshal(respBody, &value)
		if handlerErr != nil {
			log.Errorf(fmt.Sprintf("okx getExchangeInfo http err %v", handlerErr))
			return
		}
		code := value.Code
		if code != "0" {
			log.Errorf(fmt.Sprintf("okx getExchangeInfo err %v", value))
			return
		}
		// 如果可以正常解析，则保存该json 的raw信息
		fileNameJsonRaw := strings.ReplaceAll(fileName, ".json", ".rsp.json")
		helper.SaveStringToFile(fileNameJsonRaw, respBody)

		for _, data := range value.Data {
			symbol := data.InstID
			var info helper.ExchangeInfo

			info.Symbol = symbol
			info.Pair = symbolToPair(symbol)
			ctVal, err := strconv.ParseFloat(data.CtVal, 64)
			if err != nil || ctVal > fixed.BIG.Float() {
				// todo like KISHU-USDT-SWAP
				log.Warnf("ctVal too big, ignore at present %v", data)
				continue
			}
			leverExchange, _ := strconv.Atoi(data.Lever)
			info.Multi = fixed.NewF(ctVal) // 一张合约值多少基础币
			info.TickSize = helper.MustGetFloat64FromString(data.TickSz)
			info.StepSize = fixed.NewS(data.LotSz).Mul(info.Multi).Float()
			info.ContractSize = helper.MustGetFloat64FromString(data.LotSz)
			// info.StepSize = fixed.NewS(helper.MustGetShadowStringFromBytes(data, "lotSz")).Float()
			info.MaxOrderAmount = fixed.BIG
			// minSz	String	最小下单数量, 合约的数量单位是张，现货的数量单位是交易货币
			info.MinOrderAmount = fixed.NewS(data.MinSz).Mul(info.Multi)
			info.MaxOrderValue = fixed.NewF(200000.0)
			info.MinOrderValue = fixed.TEN
			info.MaxPosAmount = fixed.BIG
			info.MaxPosValue = fixed.BIG
			info.MaxLeverage = int(leverExchange)
			if leverExchange > helper.MAX_LEVERAGE {
				info.MaxLeverage = helper.MAX_LEVERAGE
			}
			if info.MaxPosAmount == fixed.NaN || info.MaxPosAmount.IsZero() {
				info.MaxPosAmount = fixed.BIG
			}

			info.Status = data.State == "live"
			if info.Pair.Equal(rs.pair) {
				helper.CopySymbolInfo(infoIn, &info)
			}
			infos = append(infos, info)
			exchangeInfoPtrP2S.Set(info.Pair.String(), &info)
			exchangeInfoPtrS2P.Set(info.Symbol, &info)
		}
	})
	if err != nil {
		log.Errorf("getExchangeInfo err %v\n", err)
		return nil, nil
	}
	if infoIn.Symbol == "" {
		log.Errorf("getExchangeInfo err, no symbol. pair: %v\n", rs.pair)
		return nil, nil
	}
	// 写入文件
	jsonData, err := json.Marshal(infos)
	if err != nil {
		for _, info := range infos {
			fmt.Println("info:", info)
		}
		log.Errorf("getExchangeInfo parse err %v\n", err)
		return nil, nil
	}
	if err := os.WriteFile(fileName, jsonData, 0644); err != nil {
		log.Errorf("getExchangeInfo write file err %v\n", err)
	}
	return infoIn, infos
}

func getWsLogin(params *helper.BrokerConfigExt) []byte {
	// login first
	ts := fmt.Sprintf("%d", GetAdjustTime().Unix())
	sign, _ := HmacSHA256Base64Sign(params.SecretKey, ts+"GET"+"/users/self/verify")
	login := map[string]interface{}{
		"op": "login",
		"args": []interface{}{
			map[string]interface{}{
				"apiKey":     params.AccessKey,
				"passphrase": params.PassKey,
				"timestamp":  ts,
				"sign":       sign,
			},
		},
	}
	msg, _ := json.Marshal(login)
	return msg

}
