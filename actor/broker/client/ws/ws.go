package ws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"actor/helper"
	"actor/helper/helper_ding"
	"actor/third/gcircularqueue_generic"
	"actor/third/log"

	"github.com/gobwas/ws"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/atomic"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var disableIpCheck = ""

type _SubStat struct {
	TryNum  int // 已尝试次数
	Success atomic.Bool
}

func (s *_SubStat) Reset() {
	s.TryNum = 0
	s.Success.Store(false)
}

type WsMsg struct {
	// opt 不传map
	Msg map[string]interface{}
	Cb  func(msg map[string]interface{}) error
}

// 定义ws结构体的成员
type WS struct {
	conn                *WsConn        //ws连接
	pingInterval        int64          // ping间隔
	writeBufferChan     chan []byte    // buffer信道
	authFunc            func() error   // 订阅函数
	subsFunc            []func() error // 订阅函数
	msgHandle           MsgHandle      // msg处理函数
	errorHandle         ErrorHandle    // 错误处理函数
	connected           atomic.Bool    // 连接标志
	wsURL               string         // url网址
	header              ws.HandshakeHeaderHTTP
	proxyURL            string              // 代理网址
	localAddr           string              // 指定本机ip
	pingFunc            func() []byte       // 发送ping函数
	pingFuncOpPing      func() []byte       // 发送ping函数with opping
	pongFunc            func([]byte) []byte // 发送pong函数
	reconnectPreHandles []func() error      // 在重连前执行的函数
	reconnectPostHandle func() error        // 在重连后执行的函数
	reconnectNum        atomic.Uint32       // 重连尝试次数
	stopCb              func(msg string)    // 需要主动停机时候的回调函数
	//logger          s.logger.Logger     // 日志实例
	lock sync.Mutex
	//
	stopCw               chan struct{}
	stopCr               chan struct{}
	subMap               sync.Map // <topic string, *_SubStat>
	excludeIpsAsPossible []string
	latestReconnectTsSec atomic.Int64
	donotRoutine         bool // 不要协程化调用msg handler
	writeChanFullNum     atomic.Int32
	reconnectting        atomic.Bool
	msgDecompresser      func([]byte) ([]byte, error)

	authSuccess       atomic.Bool
	authRetry         atomic.Int32
	reconnectSuccTsMs *gcircularqueue_generic.CircularQueueThreadSafe[int64] // 最近重连成功的时间戳
	//
	lastRecvTimeNs atomic.Int64
	lastSendTimeNs atomic.Int64
	robotId        string
	logger         log.Logger
}

// 构造函数
func NewWS(wsURL, localAddr, proxyURL string, msgHandle MsgHandle, stopCb func(msg string), params helper.BrokerConfig, excludeIpsAsPossible ...[]string) *WS {
	ws := WS{
		writeBufferChan: make(chan []byte, 100),
		conn:            nil,
		pingInterval:    30,
		msgHandle:       msgHandle,
		wsURL:           wsURL,
		proxyURL:        proxyURL,
		//logger:          s.logger.GetLogger("base_ws"),
		localAddr:         localAddr,
		stopCw:            make(chan struct{}, 10),
		stopCr:            make(chan struct{}, 10),
		stopCb:            stopCb,
		donotRoutine:      true,
		reconnectSuccTsMs: gcircularqueue_generic.NewCircularQueueThreadSafe[int64](10),
		robotId:           params.RobotId,
		logger:            params.Logger,
	}
	if len(excludeIpsAsPossible) > 0 {
		ws.excludeIpsAsPossible = excludeIpsAsPossible[0]
	}
	return &ws
}

type Configuration struct {
	donotRoutine bool // 不要协程化调用msg handler
}
type Option func(c *Configuration)

func SetDontRoutine() Option {
	return func(c *Configuration) {
		c.donotRoutine = true
	}
}
func NewWSWithOptions(wsURL, localAddr, proxyURL string, msgHandle MsgHandle, stopCb func(msg string), excludeIpsAsPossible []string, params helper.BrokerConfig, options ...Option) *WS {
	ws := NewWS(wsURL, localAddr, proxyURL, msgHandle, stopCb, params, excludeIpsAsPossible)
	c := &Configuration{}
	for _, option := range options {
		option(c)
	}
	ws.donotRoutine = c.donotRoutine
	return ws
}

func (s *WS) SetDontRoutine() {
	s.donotRoutine = true
}
func (s *WS) GetBindIp() string {
	return s.conn.bindIp
}

func (s *WS) GetWsLocalAddr() string {
	return s.conn.GetLocalAddr()
}

// 设置错误处理回调函数
func (s *WS) SetErrorHandle(errorHandle ErrorHandle) {
	s.errorHandle = errorHandle
}

func (s *WS) SetPingInterval(pingInterval int) {
	s.pingInterval = int64(pingInterval)
}

// 设置ping函数
func (s *WS) SetPingFunc(f func() []byte) *WS {
	s.pingFunc = f
	return s
}
func (s *WS) SetPingFuncWithOpPing(f func() []byte) *WS {
	s.pingFuncOpPing = f
	return s
}
func (s *WS) SetPongFunc(f func([]byte) []byte) *WS {
	s.pongFunc = f
	return s
}

func (s *WS) AddReconnectPreHandler(f func() error) *WS {
	if s.reconnectPreHandles == nil {
		s.reconnectPreHandles = make([]func() error, 0)
	}
	s.reconnectPreHandles = append(s.reconnectPreHandles, f)
	return s
}
func (s *WS) SetReconnectPostHandler(f func() error) *WS {
	s.reconnectPostHandle = f
	return s
}
func (s *WS) SetDecompressor(f func([]byte) ([]byte, error)) {
	s.msgDecompresser = f
}

// 设置ws url
func (s *WS) SetWsUrl(url string) *WS {
	s.wsURL = url
	return s
}

func (s *WS) SetHeader(header http.Header) *WS {
	s.header = ws.HandshakeHeaderHTTP(header)
	return s
}

// 设置订阅函数 仅设置 不执行
func (s *WS) SetSubscribe(cb func() error) error {
	s.subsFunc = append(s.subsFunc, cb)
	return nil
}

func (s *WS) SetAuth(cb func() error) error {
	s.authFunc = cb
	return nil
}

// 发送msg 从chan执行 延迟较高 用于不紧急场景
func (s *WS) SendMessage(msg []byte) {
	select {
	case s.writeBufferChan <- msg:
	default:
		if s.writeChanFullNum.Inc()%16 == 0 {
			helper.LogErrorThenCall(fmt.Sprintf("[%s][%p] write chan full num %d", s.wsURL, s, s.writeChanFullNum.Load()), helper_ding.DingingSendSerious)
		}
		// 让协程堵塞
		s.writeBufferChan <- msg
		s.writeChanFullNum.Store(0)
	}
}

func (s *WS) IsAllSubSuccess() bool {
	allSuccess := true
	s.subMap.Range(func(key, value any) bool {
		allSuccess = value.(*_SubStat).Success.Load()
		return allSuccess
	})
	return allSuccess
}

func AllWsSubsuccess(in ...*WS) bool {
	for _, w := range in {
		if w != nil {
			if !w.IsAllSubSuccess() {
				return false
			}
		}
	}
	return true
}

func (s *WS) SetSubSuccess(channel string) {
	s.logger.Infof("[%p] 订阅成功 url %s, channel %s", s, s.wsURL, channel)
	if sst, ok := s.subMap.Load(channel); ok {
		ss := sst.(*_SubStat)
		ss.Success.Store(true)
	}
}

func (s *WS) SubWithRetry(topic string, cb func(string), msgGen func() []byte) {
	go func() {
		sst, _ := s.subMap.LoadOrStore(topic, &_SubStat{})
		ss := sst.(*_SubStat)
		ss.Reset()
		for ; ss.TryNum <= 5 && !ss.Success.Load(); ss.TryNum++ {
			s.SendMessage(msgGen())
			time.Sleep(time.Second * 2)
		}
		if !ss.Success.Load() {
			helper.LogErrorThenCall(fmt.Sprintf("[%p][%s] 一直无法订阅 %s, 需要停机", s, s.wsURL, topic), cb)
		}
	}()
}

// 发送wsMsg结构体 可以设置 err cb
// opt 支持结构体 easyjson 序列化
func (s *WS) SendMessage2(msg WsMsg) {
	if s.connected.Load() {
		d, _ := json.Marshal(msg.Msg)
		s.lock.Lock()
		s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
		err := s.conn.Write(ws.OpText, d)
		s.lock.Unlock()
		if err != nil {
			msg.Cb(msg.Msg)
		}
		if helper.DEBUGMODE {
			s.logger.Debugf("[ws %p] [%s] 写入数据:%v, string:%s", s, s.wsURL, msg.Msg, helper.BytesToString(d))
		}
	} else {
		msg.Cb(msg.Msg) // 这里不是异步执行 里面不要放耗时操作
		// 应该直接 error log
		s.logger.Errorf("[utf_ign][ws %p] [%s] conn关闭 写入失败:%v", s, s.wsURL, msg.Msg)
	}
}

// 发送msg 立刻执行
func (s *WS) Send(msg []byte) (err error) {
	if s.conn == nil {
		return errors.New("conn is nil")
	}
	// 生产环境可以注释以节省性能
	if helper.DEBUGMODE {
		s.logger.Debugf("[ws %p] %s 写入: %s", s, s.wsURL, helper.BytesToString(msg))
	}
	s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
	return s.conn.Write(ws.OpText, msg)
}

func (s *WS) SendPingFrame() (err error) {
	if s.conn == nil {
		return errors.New("conn is nil")
	}
	s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
	return s.conn.Write(ws.OpPing, []byte("ping"))
}

// 安全连接
func (s *WS) safeDialContext(wsURL string, header ws.HandshakeHeaderHTTP, localAddr string, proxyURL string, excludeIps []string) (conn *WsConn, err error) {

	if helper.DEBUGMODE {
		if helper.HttpProxy != "" {
			parsedURL, err := url.Parse(wsURL)
			if err != nil || parsedURL.Host == "" {
				s.logger.Errorf("failed to parse url. %s", wsURL)
				return nil, err
			}
			// fixme 已经调用 ws.SetHeader的可能不适用
			header = ws.HandshakeHeaderHTTP(http.Header{
				"X-P-Host":   []string{parsedURL.Host},
				"X-P-TimeMs": []string{fmt.Sprintf("%d", time.Now().UnixMilli())},
			})
			if strings.Contains(helper.HttpProxy, ":") {
				parsedURL.Host = helper.HttpProxy
			} else {
				parsedURL.Host = helper.HttpProxy + ":443"
			}
			wsURL = parsedURL.String()
		}
	}

	for i := 0; i < 3; i++ {
		conn, err = DialContext(context.TODO(), wsURL, header, localAddr, proxyURL, 3*time.Second, excludeIps) //3s timeout
		if err == nil {
			s.logger.Infof("[ws] %s 连接成功. localAddr %s", wsURL, localAddr)
			return
		} else {
			s.logger.Warnf("[ws] %s 连接失败 %v", wsURL, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("[ws] dial timeout, wsURL:%s, wsProxy:%s", wsURL, proxyURL)
}

// 返回是否连接
func (s *WS) IsConnected() bool {
	return s.connected.Load()
}

func (s *WS) countFiles(folderPath string) (int, error) {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return -1, err
	}
	workingFilesCnt := 0
	nowNs := time.Now().UnixNano()
	for _, f := range files {
		if f.IsDir() {
			// push.PushAlert("base ws reconnect", fmt.Sprintf("should not have dir in reconnect tmp folder. %s", f.Name()))
			continue
		}
		// 会打印不包含路径文件名
		tsns, err := strconv.Atoi(f.Name())
		if err != nil {
			// push.PushAlert("base ws reconnect", fmt.Sprintf("reconnect tmp file name is not a number. %s", f.Name()))
		}
		if int64(tsns)+(time.Minute*5).Nanoseconds() < nowNs {
			s.logger.Warnf("too old reconnect tmp file. %s", f.Name())
			if int64(tsns)+(time.Minute*60).Nanoseconds() < nowNs {
				s.logger.Warnf("remove huge old reconnect tmp file. %s", filepath.Join(folderPath, f.Name()))
				s.removeFile(filepath.Join(folderPath, f.Name()))
			}
		} else {
			workingFilesCnt += tsns
		}
	}
	return workingFilesCnt, err
}

func (s *WS) removeFile(filePath string) (err error) {
	err = os.Remove(filePath)
	if err != nil {
		s.logger.Errorf("failed to remove ws holding file. %s", err.Error())
		return
	}
	s.logger.Infof("ws reconnect tmp file removed: [%s]", filePath)
	return
}

func (s *WS) Reconnect() (err error) {
	if s.reconnectting.Load() {
		s.logger.Info("[ws %p] reconnecting, ignore this one", s)
		return
	}
	if time.Now().Unix()-s.latestReconnectTsSec.Load() < 10 {
		s.logger.Warnf("[ws %p] can reconnect one times in 10 sec", s)
		return
	}

	url := s.wsURL[6:]
	// 用listen key 的所需要特殊处理
	if strings.Contains(url, "binance") || strings.Contains(url, "kucoin") {
		idx := strings.LastIndex(url, "/")
		if idx > 0 {
			url = url[:idx]
		}
	}
	url = strings.ReplaceAll(url, "/", "-")
	url = strings.ReplaceAll(url, ":", "-")
	basePath := fmt.Sprintf("/tmp/bqws.reconnect/%s/%s", helper.KeepOnlyNumbers(helper.BuildTime), url)
	if s.localAddr != "" {
		basePath = fmt.Sprintf("/tmp/bqws.reconnect/%s/%s/%s", helper.KeepOnlyNumbers(helper.BuildTime), s.localAddr, url)
	}

	_, err = os.Stat(basePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			s.logger.Errorf("failed to create ws temp folder: [%v]", err)
			return
		}
	}

	len := 0
	tried := 0
	MAX := 15
	for tried <= MAX {
		tried++
		len, err = s.countFiles(basePath)
		if err != nil {
			s.logger.Errorf("failed to read ws tmp folder: [%v]", err)
			return
		}
		if len < 3 {
			break
		}
		s.logger.Warnf("[ws %p] too many ws reconnecting, waiting %s ...", s, basePath)
		time.Sleep(time.Second * 2)
	}
	if tried >= MAX {
		return fmt.Errorf("[ws %p] not free quote for ws reconnect final", s)
	}

	timestamp := time.Now().UnixNano()
	filePath := fmt.Sprintf("%s/%d", basePath, timestamp)

	_, err = os.Create(filePath)
	if err != nil {
		s.logger.Errorf("[ws %p] failed to create ws reconnect tmp file: [%v]", s, err)
		return
	}
	defer s.removeFile(filePath)
	s.logger.Infof("[ws %p] reconnect tmp file created: [%s]", s, filePath)

	err = s.reconnect()
	if err != nil {
		s.logger.Errorf("[ws %p] failed reconnect Ws: [%s][%v]", s, filePath, err)
		return
	}

	return err
}

// 重连函数
// 创建新的conn 重新执行一遍订阅 重新执行一遍连接成功回调函数
func (s *WS) reconnect() (err error) {
	for _, f := range s.reconnectPreHandles {
		f()
	}
	s.latestReconnectTsSec.Store(time.Now().Unix())

	s.reconnectNum.Add(1)
	s.reconnectting.Store(true)
	defer s.reconnectting.Store(false)
	if s.reconnectNum.Load() > 10 {
		ds := fmt.Sprintf("%dms", int(2*s.reconnectNum.Load()*1000/10))
		d, err := time.ParseDuration(ds)
		if err != nil {
			s.logger.Errorf("[ws %p] 重连无法解析重连间隔时间:%s", s, ds)
		} else {
			s.logger.Warnf("[ws %p] 重连次数太多，准备等待间隔时间:%s", s, ds)
			time.Sleep(d)
		}
	}
	s.logger.Warnf("[ws %p] 第%v次尝试重连...", s, s.reconnectNum.String())
	conn, err := s.safeDialContext(s.wsURL, s.header, s.localAddr, s.proxyURL, s.excludeIpsAsPossible)
	if err != nil {
		s.logger.Errorf("[ws %p] [%s] 自动重连失败 :%s", s, s.wsURL, err.Error())
		s.handleErr(fmt.Errorf("[ws %p] [%s] 自动重连失败 :%s", s, s.wsURL, err.Error()), true)
		s.reconnectting.Store(false)
		return
	} else {
		s.conn.Close()
		s.conn = conn // new connect
		s.connected.Store(true)
		s.reconnectting.Store(false)
		s.logger.Warnf("[ws %p][%s]自动重连成功 ", s, s.wsURL)
		// 2分钟内重连次数太多，退出停机。有可能交易所在连接成功后马上断连接
		s.reconnectSuccTsMs.PushKick(time.Now().UnixMilli())
		if s.reconnectSuccTsMs.IsFull() {
			len := s.reconnectSuccTsMs.Len()
			last := s.reconnectSuccTsMs.GetElement(len - 1)
			first := s.reconnectSuccTsMs.GetElement(0)
			if last-first < 2*60*1000 {
				msg := fmt.Sprintf("[ws %p] too many ws reconnections within short time secs: %d ", s, (last-first)/1000)
				helper.LogErrorThenCall(msg, s.stopCb)
				return
			}
		}
		s.reconnectNum.Store(0)
		nowNs := time.Now().UnixNano()
		// 重连成功后等同链路激活
		s.lastRecvTimeNs.Store(nowNs)
		s.lastSendTimeNs.Store(nowNs)
		if s.reconnectPostHandle != nil {
			s.reconnectPostHandle()
		}

		// 重新订阅
		if s.authFunc != nil {
			s.authSuccess.Store(false)
			s.authRetry.Store(0)
			go func() {
				for {
					if s.authSuccess.Load() {
						for _, subFun := range s.subsFunc {
							go subFun()
						}
						break
					} else if s.authRetry.Load() > 3 {
						helper.LogErrorThenCall(fmt.Sprintf("[ws %p][%s] 一直无法Auth during reconnect, 需要停机", s, s.wsURL), s.stopCb)
						return
					} else {
						go s.authFunc()
						s.authRetry.Inc()
						time.Sleep(time.Second * 2)
					}
				}
			}()
		} else {
			s.authSuccess.Store(true)
			for _, subFun := range s.subsFunc {
				go subFun()
			}
		}
	}
	return
}

const _MAX_LEN_POOL_ITEM = 1024 // bytes

type RespPool struct {
	pool      sync.Pool
	maxLen    int
	dropTimes int // 丢弃大元素的次数，如果次数相对前值增加很大，说明内存利用效率低
}

func (p *RespPool) Get() *bytes.Buffer {
	r := p.pool.Get()
	if r == nil {
		return bytes.NewBuffer(make([]byte, 0, p.maxLen))
	}
	return r.(*bytes.Buffer)
}
func (p *RespPool) Put(b *bytes.Buffer) {
	cap := b.Cap()
	if cap > _MAX_LEN_POOL_ITEM {
		b.Reset()
		p.dropTimes++
		return // 超过大小不入池
	}
	if cap > p.maxLen {
		p.maxLen = cap
	}
	b.Reset()
	p.pool.Put(b)
}

var respPool *RespPool = &RespPool{
	pool:   sync.Pool{},
	maxLen: 128,
}

func (s *WS) SetAuthSuccess() {
	s.authSuccess.Store(true)
}
func (s *WS) IsAuthSuccess() bool {
	return s.authSuccess.Load()
}

// 启动ws
func (s *WS) Serve() (stopC chan struct{}, err error) {
	// 创建连接
	s.conn, err = s.safeDialContext(s.wsURL, s.header, s.localAddr, s.proxyURL, s.excludeIpsAsPossible)
	if err != nil {
		// 如果连接失败
		s.logger.Errorf("[ws %p]连接失败 %s, %v", s, s.wsURL, err)
		return
	} else {
		// 连接成功
		s.connected.Store(true)
	}
	// 接受结束和停止信号
	stopC = make(chan struct{}, 10)

	// 处理退出逻辑的协程
	go func() {
		select {
		case <-stopC:
			s.connected.Store(false)
			s.stopCw <- struct{}{}
			s.stopCr <- struct{}{}
			// do exit
			time.Sleep(time.Second)
			err = s.close()
			if err != nil {
				s.logger.Errorf("[ws %p]尝试close退出失败 %v", s, err.Error())
				s.handleErr(err, true)
			}
			return
		}
	}()

	/* -------------- 写入ws -------------- */
	go func() {
		//Auth + 订阅
		if s.authFunc != nil {
			s.authSuccess.Store(false)
			s.authRetry.Store(0)
			go func() {
				for {
					if s.reconnectting.Load() {
						return
					}
					if s.authSuccess.Load() {
						for _, subFun := range s.subsFunc {
							go subFun()
						}
						break
					} else if s.authRetry.Load() > 3 {
						helper.LogErrorThenCall(fmt.Sprintf("[ws %p][%s] 一直无法Auth during Serve, 需要停机", s, s.wsURL), s.stopCb)
						return
					} else {
						go s.authFunc()
						s.authRetry.Inc()
						time.Sleep(time.Second * 2)
					}
				}
			}()
		} else {
			s.authSuccess.Store(true)
			for _, subFun := range s.subsFunc {
				go subFun()
			}
		}

		pingTick := time.NewTicker(time.Second * time.Duration(s.pingInterval))
		defer pingTick.Stop()

		timeoutNs := 3 * s.pingInterval * 1e9

		stopTick := time.NewTicker(time.Second * time.Duration(60))
		defer stopTick.Stop()

		for {
			select {
			case <-s.stopCw: // 收到停机信息时 退出
				s.logger.Warnf("退出ws写入loop. %s", s.wsURL)
				return
			case <-pingTick.C: // 循环主动ping 如果存在ping func的话
				if s.lastRecvTimeNs.Load() > 0 || s.lastSendTimeNs.Load() > 0 {
					nowNs := time.Now().UnixNano()
					// 有些所下单链接能收但不回任何消息，所以通过 SendTimeNs 判断
					if ((nowNs - s.lastRecvTimeNs.Load()) > timeoutNs) && ((nowNs - s.lastSendTimeNs.Load()) > timeoutNs) {
						s.logger.Errorf("ws %p %s 不活动数据超过ping间隔时间x3", s, s.wsURL)
						s.Reconnect()
						continue
					}
				}
				if s.pingFunc != nil {
					// if helper.DEBUGMODE {
					// s.logger.Debugf("[ws][Ping]")
					// }
					s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
					err = s.conn.Write(ws.OpText, s.pingFunc())
					if err != nil {
						s.logger.Errorf("[ws %p] [%s] ping失败 :%s", s, s.wsURL, err.Error())
						s.handleErr(fmt.Errorf("[ws %p] [%s] ping失败 :%s", s, s.wsURL, err.Error()), true)
					}
				}
				if s.pingFuncOpPing != nil {
					if helper.DEBUGMODE {
						// s.logger.Debugf("[ws][Ping with OpPing]")
					}
					s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
					err = s.conn.Write(ws.OpPing, s.pingFuncOpPing())
					if err != nil {
						s.logger.Errorf("[ws %p] [%s] ping失败 :%s", s, s.wsURL, err.Error())
						s.handleErr(fmt.Errorf("[ws %p] [%s] ping失败 :%s", s, s.wsURL, err.Error()), true)
					}
				}
			case <-stopTick.C: // 检查是否需要主动停机
				// 检查重连次数是否异常
				if s.reconnectNum.Load() > 10 {
					s.stopCb("ws重连次数过多 异常停机")
				}
			case d := <-s.writeBufferChan: // channel收到待发送信息时
				s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
				err = s.conn.Write(ws.OpText, d)
				if err != nil {
					s.logger.Errorf("[ws %p] [%s] 写入失败 :%s", s, s.wsURL, err.Error())
					s.handleErr(fmt.Errorf("[ws] [%s]write data fail:%s", s.wsURL, err.Error()), true)
				} else {
					s.lastSendTimeNs.Store(time.Now().UnixNano())
				}
				// 生产环境可以注释以节省性能
				if helper.DEBUGMODE {
					s.logger.Debugf("[ws %p] [%s] 写入成功 :%s", s, s.wsURL, helper.BytesToString(d))
				}
			}
		}
	}()

	/* -------------- 读取ws -------------- */
	go func() {
		runtime.LockOSThread()
		// 等待触发
		for {
			// 底层是epoll 没有cpu轮询
			select {
			case <-s.stopCr: // 收到停机信息时 退出
				s.logger.Warnf("退出ws读取loop, %s", s.wsURL)
				s.logger.Warnf("丢弃大元素次数 dropTimes %d", respPool.dropTimes)
				return
			default:
				// 读取信息
				var op ws.OpCode
				msgBuf := respPool.Get()
				err := s.conn.Read(&op, msgBuf)
				// 记录读取信息的时间戳
				ts := time.Now().UnixNano()
				// 如果读取出错 尝试重连
				if err != nil {
					respPool.Put(msgBuf)
					if opError, ok := err.(*net.OpError); ok && opError.Timeout() {
						continue
					}
					if !s.connected.Load() && strings.Contains(err.Error(), "use of closed network connection") {
						// 主动断开的链接忽略错误
					} else {
						willReconnect := s.connected.Load()
						s.handleErr(err, !willReconnect) // 常规重连不需要err log
					}
					if s.connected.Load() {
						// okx出现重连成功但马上EOF的情况，会频繁打印并调用reconnectPreHandle和Reconnect。用10s过滤一下
						if s.reconnectting.Load() || time.Now().Unix()-s.latestReconnectTsSec.Load() < 10 {
							time.Sleep(time.Second) //  conn.Read会频繁读取出现断网提示
							continue
						}

						// 如果ws还没有停止 允许重连
						s.logger.Warnf("[ws %p] [%s] 意外关闭 开始重连 错误内容:%v", s, s.wsURL, err.Error())
						if err := s.Reconnect(); err != nil && s.reconnectNum.Load() > 10 {
							s.stopCb("ws重连失败 异常停机. " + err.Error())
							return
						}
					}
					continue
				}
				if s.donotRoutine {
					// 有些信息中间状态不可丢，且交易所没有提供seq\检验机制，但推送有序，可以串行化处理
					s.handleMsg(op, msgBuf, ts)
				} else {
					// 必须异步 提高极限性能
					// 当需要强制处理顺序时 使用ts字段进行排序
					// pos equity 对处理顺序敏感
					go s.handleMsg(op, msgBuf, ts)
				}
			}
		}
	}()

	return
}

var _WRITE_TIMEOUT = time.Second * 2

func (s *WS) handleMsg(op ws.OpCode, msgBuf *bytes.Buffer, ts int64) {
	// if !helper.DEBUGMODE {
	// 	s.doHandleMsg(op, msgBuf, ts, nil)
	// 	return
	// }

	// DEBUGMODE 模式开启测试
	// timeout := 10 * time.Second
	// ctx, cancel := context.WithCancel(context.Background())
	// go func() {
	// 	select {
	// 	case <-time.After(timeout):
	// 		buf := make([]byte, 1<<20) // 1 MB buffer
	// 		stacklen := runtime.Stack(buf, true)
	// 		aa := string(buf[:stacklen])
	// 		s.logger.Warnf("ws rsp handler timeout, msg %s", helper.BytesToString(msgBuf.Bytes()))
	// 		s.logger.Warnf("[线程堆栈] %v", aa)
	// 		helper.PushAlert("ws rsp handler timeout", s.robotId)
	// 	case <-ctx.Done():
	// 		return
	// 	}
	// }()
	s.doHandleMsg(op, msgBuf, ts, nil)
}
func (s *WS) doHandleMsg(op ws.OpCode, msgBuf *bytes.Buffer, ts int64, cancelFunc context.CancelFunc) {
	s.lastRecvTimeNs.Store(ts)
	msg := msgBuf.Bytes()
	if helper.DEBUGMODE && cancelFunc != nil {
		defer cancelFunc()
	}
	defer respPool.Put(msgBuf)
	respHandledOK := false
	defer func() {
		if !respHandledOK {
			if s.msgDecompresser != nil {
				msg, _ = s.msgDecompresser(msg)
			}
			s.logger.Errorf("[ws %p] msgHandler error, please check struct field. Resp: %v\n", s, string(msg))
			if err := recover(); err != nil {
				s.logger.Error(err)
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				s.logger.Errorf("==> %s\n", string(buf[:n]))
			}
		}
	}()
	switch op {
	case ws.OpPing:
		s.conn.conn.SetWriteDeadline(time.Now().Add(_WRITE_TIMEOUT))
		if s.pongFunc != nil {
			bytes := s.pongFunc(msg)
			s.conn.Write(ws.OpPong, bytes)
			if helper.DEBUGMODE {
				// s.logger.Debugf("[ws][%s][Recv OpPing and write OpPong] ping: %v pong: %v\n", s.wsURL, string(msg), string(bytes))
			}
		} else {
			s.conn.Write(ws.OpPong, nil)
			if helper.DEBUGMODE {
				// s.logger.Debugf("[ws][%s][Recv OpPing and write OpPong] %v\n", s.wsURL, string(msg))
			}
		}
	case ws.OpPong:
		if helper.DEBUGMODE {
			// s.logger.Debugf("[ws][%s][Recv OpPong] %v\n", s.wsURL, string(msg))
		}
	case ws.OpContinuation:
		// 生产环境可以注释以节省性能
		//if helper.DEBUGMODE {
		//	s.logger.Debugf("[ws][%s]%v", s.wsURL, helper.BytesToString(msg))
		//}
		s.msgHandle(msg, ts)
	case ws.OpText:
		// 生产环境可以注释以节省性能
		//if helper.DEBUGMODE {
		//	s.logger.Debugf("[ws][%s][%v]%v", s.wsURL, helper.BytesToString(msg), ts)
		//}
		s.msgHandle(msg, ts)
	case ws.OpBinary:
		s.msgHandle(msg, ts)
	default:
	}
	respHandledOK = true
}

// 关闭连接
func (s *WS) close() error {
	s.logger.Infof("[ws] 准备关闭")
	// 关闭连接
	err := s.conn.Close()
	if err != nil {
		s.logger.Errorf("[ws] 尝试关闭失败 %v", err)
	} else {
		s.logger.Infof("[ws] 关闭成功")
	}
	s.connected.Store(false)
	return err
}

// 处理异常 如果有异常处理回调函数则执行 如果没有就记录日志
func (s *WS) handleErr(err error, errPrint bool) {
	if errPrint {
		s.logger.Errorf("[utf_ign][ws %p][%s] 出现错误: %v", s, s.wsURL, err)
	} else {
		s.logger.Warnf("[utf_ign][ws %p][%s] 出现错误: %v", s, s.wsURL, err)
	}
	if s.errorHandle != nil {
		go s.errorHandle(s.wsURL, err)
	}
}
