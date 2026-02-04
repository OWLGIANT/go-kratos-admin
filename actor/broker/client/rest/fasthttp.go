// !!!DON'T TRY TO USING HTTP/2 IN FASTHTTP!!! IT'LL PANIC!!!

package rest

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"runtime"
	"strings"
	"time"

	"actor/broker/base"
	"actor/helper"
	"actor/third/log"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

const (
	socks5ProxyPrefix = "socks5://"
	httpProxyPrefix   = "http://"

	defaultReadBufferSize  = 4096 * 8
	defaultWriteBufferSize = 4096 * 8
)

var (
	DefaultTimeout             = 10 * time.Second
	DefaultMaxIdleConnDuration = 90 * time.Second
	disableIpCheck             = ""
)

type FastHttpRespHandler func(respBody []byte, respHeader *fasthttp.ResponseHeader)

var ipMap = make(map[string]string, 0) // pri:pub
var pubIpSet = make(map[string]struct{}, 0)

//func ensureUseCorrectIp() bool {
//	if len(ipMap) == 0 {
//		return true
//	}
//	mi := brokerconfig.LoadMachineInfo()
//	if mi.Zone == "qq-sg-office" || mi.IgnoreIpCheck {
//		return true
//	}
//	logger := log.InitWithLogger("/tmp/fh.log", "debug")
//	for k, pubIp := range ipMap {
//		client := NewClient("", k, logger)
//		success := false
//		statusCode, err := client.Request("GET", "http://auth.thousandquant.com:8500/ips", nil, nil, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
//			success = strings.Contains(string(respBody), pubIp)
//			if !success {
//				logger.Errorf("failed to check ip: %s. rsp: %s", pubIp, string(respBody))
//			}
//		})
//		if err != nil {
//			logger.Errorf("failed to request ips: %v", err)
//			return false
//		}
//		if statusCode != 200 {
//			logger.Errorf("failed to request ips: %v", statusCode)
//			return false
//		}
//		if !success {
//			return false
//		}
//	}
//	return true
//}

type Client struct {
	client    *fasthttp.Client
	timeout   time.Duration
	logger    log.Logger
	userAgent string
}

// 构造函数
func NewClient(proxyURL, localAddr string, logger log.Logger) *Client {
	timeout := DefaultTimeout
	// 实例化fasthttp的client
	client := &fasthttp.Client{
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		MaxConnsPerHost:               2000,
		MaxIdleConnDuration:           DefaultMaxIdleConnDuration,
		ReadTimeout:                   timeout,
		WriteTimeout:                  timeout,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		},
		ReadBufferSize:  defaultReadBufferSize,
		WriteBufferSize: defaultWriteBufferSize,
	}
	if localAddr != "" {
		client.Dial = func(addr string) (conn net.Conn, e error) {
			lAddr, err := net.ResolveTCPAddr("tcp", localAddr+":0")
			if err != nil {
				return nil, err
			}
			if helper.DEBUGMODE {
				logger.Debug("reading ip for: ", addr)
			}
			touched := helper.TouchIpToQuery(addr)
			var rAddr *net.TCPAddr
			if touched {
				fastIp, port := helper.GetFastIp(addr)
				if fastIp == "" {
					rAddr, err = net.ResolveTCPAddr("tcp", addr)
					if err != nil {
						return nil, err
					}
				} else {
					rAddr = &net.TCPAddr{
						IP:   net.ParseIP(fastIp),
						Port: port,
					}
					logger.Infof("will use fastip %s for: %s ", fastIp, addr)
				}
			} else {
				rAddr, err = net.ResolveTCPAddr("tcp", addr)
				if err != nil {
					return nil, err
				}
			}
			conn, err = net.DialTCP("tcp", lAddr, rAddr)
			if err != nil {
				return nil, err
			}
			return conn, nil
		}
	} else {
		switch {
		case strings.HasPrefix(proxyURL, socks5ProxyPrefix):
			client.Dial = fasthttpproxy.FasthttpSocksDialer(proxyURL)
		case strings.HasPrefix(proxyURL, httpProxyPrefix):
			proxyAddr := proxyURL[len(httpProxyPrefix):]
			client.Dial = fasthttpproxy.FasthttpHTTPDialer(proxyAddr)
		default:
			client.Dial = func(addr string) (conn net.Conn, err error) {
				if helper.DEBUGMODE {
					logger.Debug("reading ip for: ", addr)
				}
				touched := helper.TouchIpToQuery(addr)
				if touched {
					fastIp, port := helper.GetFastIp(addr)
					if fastIp == "" {
						var rAddr *net.TCPAddr
						rAddr, err = net.ResolveTCPAddr("tcp", addr)
						conn, err = net.DialTCP("tcp", nil, rAddr)
					} else {
						logger.Infof("will use fastip %s for: %s ", fastIp, addr)
						conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", fastIp, port))
					}
				} else {
					var rAddr *net.TCPAddr
					rAddr, err = net.ResolveTCPAddr("tcp", addr)
					conn, err = net.DialTCP("tcp", nil, rAddr)
				}
				if err != nil {
					return nil, err
				}
				return conn, nil
			}
		}
	}

	return &Client{
		client:  client,
		timeout: timeout,
		logger:  logger,
	}
}

func (c *Client) SetUserAgentSlice(userAgentSlice []string) {
	randomIdx := rand.Intn(len(userAgentSlice))
	c.userAgent = userAgentSlice[randomIdx]
}

func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.client.WriteTimeout = timeout
	c.client.ReadTimeout = timeout
}

// 重新实现request函数
func (c *Client) Request(reqMethod string, reqUrl string, reqBody []byte, reqHeaders map[string]string, respHandler FastHttpRespHandler, apiErr ...*helper.ApiError) (int, error) {

	// 从连接池获取conn
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	// 设置请求方法 get post delete
	req.Header.SetMethod(reqMethod)
	// 增加本次请求额外的headers
	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}
	if c.userAgent != "" {
		req.Header.SetUserAgent(c.userAgent)
	}

	// 设置本次请求的 url
	req.SetRequestURI(reqUrl)

	if helper.DEBUGMODE {
		if helper.HttpProxy != "" {
			parsedURL, err := url.Parse(reqUrl)
			if err != nil || parsedURL.Host == "" {
				c.logger.Errorf("failed to parse url. %s", req.RequestURI())
				return 400, err
			}
			req.Header.Set("X-P-Host", fmt.Sprintf("%s:%s", parsedURL.Host, parsedURL.Port()))
			req.Header.Set("X-P-TimeMs", fmt.Sprintf("%d", time.Now().UnixMilli()))
			if strings.Contains(helper.HttpProxy, ":") {
				parsedURL.Host = helper.HttpProxy
			} else {
				parsedURL.Host = helper.HttpProxy + ":443"
			}
			req.SetRequestURI(parsedURL.String())
		}
	}

	// 如果有body 就设置body
	if len(reqBody) > 0 {
		req.SetBody(reqBody)
	}

	// 从response池中获取resp
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	//生产环境可以注释以节省性能
	if helper.DEBUGMODE {
		if helper.DEBUG_PRINT_HEADER {
			c.logger.Debugf("REQ: %s %s, body:%s, header: %s", reqMethod, reqUrl, helper.BytesToString(reqBody), req.Header.String())
		} else {
			c.logger.Debugf("REQ: %s %s, body:%s", reqMethod, reqUrl, helper.BytesToString(reqBody))
		}
		// 计算报文长度
		// var buf bytes.Buffer
		// writer := bufio.NewWriter(&buf)
		// if err := req.Write(writer); err != nil {
		// c.logger.Errorf("failed to write req to buf")
		// }
		// c.logger.Debugf("http msg req len: %d; header content len:%d", writer.Buffered(), req.Header.ContentLength())
	}
	// 开始请求
	// if err := c.client.DoTimeout(req, resp, c.timeout); err != nil {
	// return -1, err
	// }
	// 有些交易所返回400 err时也返回有意义的json body，所以都应调用 respHandler，不应提前返回
	// err := c.client.DoTimeout(req, resp, c.timeout)
	// 在 doNonNilReqResp()@fasthttp/client.go会使用 WriteTimeout ReadTimeout设置超时
	err := c.client.DoRedirects(req, resp, 3)
	if err != nil {
		c.logger.Errorf("fh req DoTimeout err:%w", err)
	}

	//生产环境可以注释以节省性能
	//if helper.DEBUGMODE || base.IsInUPC {
	//	if helper.DEBUG_PRINT_HEADER {
	//		c.logger.Infof("Complete Http Msg\nREQ:[%s] %s, body:%s, header:%sRESP:[%d] \nheader:%sbody: %v\n\n", reqMethod, reqUrl, helper.BytesToString(reqBody), req.Header.String(), resp.StatusCode(), resp.Header.String(), helper.BytesToString(resp.Body()))
	//	} else {
	//		c.logger.Infof("Complete Http Msg\nREQ:[%s] %s, body:%sRESP:[%d] body: %v", reqMethod, reqUrl, helper.BytesToString(reqBody), resp.StatusCode(), helper.BytesToString(resp.Body()))
	//	}
	//}

	// 不要有复制
	// 复制原因是 resp的生命周期出了这个函数就没有了
	// 解决办法 request函数应该传入一个cb func
	// 在 cb func 中传入 resp.body() 然后触发相应的逻辑处理函数
	// 用完之后 才返回这个函数触发 defer release(resp)
	//respBody := resp.Body()
	//bodyBytes := make([]byte, len(respBody))
	//copy(bodyBytes, respBody)
	//
	//respHeader := resp.Header.Header()
	//headerBytes := make([]byte, len(respHeader))
	//copy(headerBytes, respHeader)

	//respHeader := &fasthttp.ResponseHeader{}
	//resp.Header.CopyTo(respHeader)

	if respHandler != nil {
		respHandledOK := false
		defer func() {
			if !respHandledOK {
				c.logger.Errorf("rs respHandler error, please check struct field. Request: %v\nResponse: %v\n", req, resp)
				if err := recover(); err != nil {
					if len(apiErr) > 0 {
						apiErr[0].HandlerError = fmt.Errorf("%v", err)
					}
					c.logger.Error(err)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					c.logger.Errorf("==> %s\n", string(buf[:n]))
				}
			}
		}()
		body := resp.Body()
		respHandler(body, &resp.Header)
		// if len(body) > 50*1024 { //
		// 	msg := fmt.Sprintf("%.6f kb. REQ url %s, body %s", float64(len(body))/1024.0, reqUrl, string(reqBody))
		// 	helper.PushAlert("rsp too large", msg)
		// }
		respHandledOK = true
	}
	return resp.StatusCode(), err
}

func (c *Client) RequestPure(req *fasthttp.Request, respHandler FastHttpRespHandler, needRedirect ...bool) (int, error) {

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if helper.DEBUGMODE {
		if helper.DEBUG_PRINT_HEADER {
			c.logger.Debugf("REQ: %s %s, body:%s\nheader: %s", req.Header.Method(), req.RequestURI(), helper.BytesToString(req.Body()), req.Header.String())
		} else {
			c.logger.Debugf("REQ: %s %s, body:%s", req.Header.Method(), req.RequestURI(), helper.BytesToString(req.Body()))
		}
	}

	if helper.DEBUGMODE {
		if helper.HttpProxy != "" {
			parsedURL, err := url.Parse(helper.BytesToString(req.RequestURI()))
			if err != nil || parsedURL.Host == "" {
				c.logger.Errorf("failed to parse url. %s", req.RequestURI())
				return 400, err
			}
			req.Header.Set("X-P-Host", fmt.Sprintf("%s:%s", parsedURL.Host, parsedURL.Port()))
			req.Header.Set("X-P-TimeMs", fmt.Sprintf("%d", time.Now().UnixMilli()))
			if strings.Contains(helper.HttpProxy, ":") {
				parsedURL.Host = helper.HttpProxy
			} else {
				parsedURL.Host = helper.HttpProxy + ":443"
			}
			req.SetRequestURI(parsedURL.String())
		}
	}
	if req.Header.UserAgent() == nil && c.userAgent != "" {
		req.Header.SetUserAgent(c.userAgent)
	}
	if needRedirect != nil && needRedirect[0] {
		// 在 doNonNilReqResp()@fasthttp/client.go会使用 WriteTimeout ReadTimeout设置超时
		if err := c.client.DoRedirects(req, resp, 3); err != nil {
			c.logger.Errorf("fh client do redirect error: %v", err)
			return -1, err
		}
	} else {
		if err := c.client.DoTimeout(req, resp, c.timeout); err != nil {
			c.logger.Errorf("fh client do error: %v", err)
			return -1, err
		}
	}

	if helper.DEBUGMODE || base.IsInUPC {
		if helper.DEBUG_PRINT_HEADER {
			c.logger.Infof("Complete Http Msg\nPURE REQ:[%s] %s, header:%s\nbody:%s\nRESP:[%d] \nheader:%sbody: %v\n\n", req.Header.Method(), req.RequestURI(), req.Header.String(), helper.BytesToString(req.Body()), resp.StatusCode(), resp.Header.String(), helper.BytesToString(resp.Body()))
		} else {
			c.logger.Infof("Complete Http Msg\nPURE REQ:[%s] %s, body:%s\nRESP:[%d] body: %v.\n\n", req.Header.Method(), req.RequestURI(), helper.BytesToString(req.Body()), resp.StatusCode(), helper.BytesToString(resp.Body()))
		}
	}
	if resp.StatusCode() >= 300 {
		// %v Request打印不显示host
		c.logger.Errorf("resp status code error, Request: %v Response: %v\n", req, resp)
	}

	if respHandler != nil {
		// 用于json结构读取出错时快速定位问题
		respHandledOK := false
		defer func() {
			if !respHandledOK {
				c.logger.Errorf("rs respHandler error, please check struct field. Request: %v\nResponse: %v\n", req, resp)
				if err := recover(); err != nil {
					c.logger.Error(err)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					c.logger.Errorf("==> %s\n", string(buf[:n]))
				}
			}
		}()

		body := resp.Body()
		respHandler(body, &resp.Header)
		respHandledOK = true
		// if len(body) > 50*1024 { //
		// 	msg := fmt.Sprintf("%.6f kb. REQ url %s, body %s", float64(len(body))/1024.0, req.RequestURI(), string(req.Body()))
		// 	helper.PushAlert("rsp too large", msg)
		// }
	}
	return resp.StatusCode(), nil
}
