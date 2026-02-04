package okx_spot

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
	"strings"
	"time"

	"actor/broker/brokerconfig"

	"actor/broker/client/rest"
	"actor/helper"
	"actor/third/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	okxWsPubUrl  = "wss://ws.okx.com:8443/ws/v5/public"
	okxWsPriUrl  = "wss://ws.okx.com:8443/ws/v5/private"
	okxRestUrl   = "https://www.okx.com"
	okxSimHeader = map[string]interface{}{}
)

var (
	wsPublicHandyPool  fastjson.ParserPool
	wsPrivateHandyPool fastjson.ParserPool
	handyPool          fastjson.ParserPool

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	GET  = http.MethodGet
	POST = http.MethodPost
)

// symbolToPair 交易所规范的symbol转换为内部数据结构 btc_usdt to BTC-USDT
func symbolToPair(symbol string) helper.Pair {
	p := strings.Split(symbol, "-")
	if len(p) == 2 {
		p0 := helper.Pair{
			Base:  strings.ToLower(p[0]),
			Quote: strings.ToLower(p[1]),
			More:  "",
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
		return false
	}
	cfg := brokerconfig.BrokerSession()
	if cfg.OkxUsdtSpotWsPubUrl != "" {
		okxWsPubUrl = cfg.OkxUsdtSpotWsPubUrl
		log.Infof(" ws启用colo pub %v", okxWsPubUrl)
		flag = true
	}
	if cfg.OkxUsdtSpotWsPriUrl != "" {
		okxWsPriUrl = cfg.OkxUsdtSpotWsPriUrl
		log.Infof("ws启用colo pri %v", okxWsPriUrl)
		flag = true
	}
	if cfg.OkxUsdtSpotRestUrl != "" {
		okxRestUrl = cfg.OkxUsdtSpotRestUrl
		log.Infof("rest启用colo %v", okxRestUrl)
		flag = true
	}
	if cfg.OkxUsdtSpotProxy != "" {
		proxies := strings.Split(cfg.OkxUsdtSpotProxy, ",")
		rand.Seed(time.Now().Unix())
		i := rand.Int31n(int32(len(proxies)))
		proxy := proxies[i]
		params.ProxyURL = proxy
		log.Infof("ws 启用代理: %v", params.ProxyURL)
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

type OkxSpotClient struct {
	restUrl string                  // rest地址
	params  *helper.BrokerConfigExt // 配置
	pair    helper.Pair
	client  *rest.Client // 通用rest客户端
	cb      helper.CallbackFunc
}

func NewClient(params *helper.BrokerConfigExt, cb helper.CallbackFunc) *OkxSpotClient {
	return &OkxSpotClient{
		restUrl: okxRestUrl,
		params:  params,
		client:  rest.NewClient(params.ProxyURL, params.LocalAddr, params.Logger),
		cb:      cb,
	}
}

func (c *OkxSpotClient) get(reqUrl string, params map[string]interface{}, needSign bool, respHandler rest.FastHttpRespHandler) (int, error) {
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

func (c *OkxSpotClient) post(reqUrl string, params map[string]interface{}, respHandler rest.FastHttpRespHandler) (int, error) {
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

func (c *OkxSpotClient) updateTime() bool {
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
