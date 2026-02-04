package ws

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"actor/helper"
	"actor/third/log"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"golang.org/x/net/proxy"
)

const (
	socks5ProxyPrefix = "socks5://"
	httpProxyPrefix   = "http://"
)

// const writeSize = 15000000 // 15mb
const wsWriteBufferSize = 1 << 20 // bytes单位 1<<20 = 1M

// adapted from https://github.com/chromedp/chromedp/blob/8e0a16689423d48d8907c62a543c7ea468059228/conn.go
// https://github.com/wirepair/gcd/blob/e103f957a3d72ef627143e1474d0940ce6b04e74/wsconn.go
type WsConn struct {
	conn        net.Conn
	writer      *wsutil.Writer
	reader      *wsutil.Reader
	readTimeout time.Duration
	pool        sync.Pool
	bindIp      string // 链接绑定到的IP
	//readBuffer
}

func (c *WsConn) GetLocalAddr() string {
	if c.conn != nil {
		return c.conn.LocalAddr().String()
	}
	return ""
}
func (c *WsConn) Read(op *ws.OpCode, b *bytes.Buffer) error {
	c.conn.SetReadDeadline(time.Now().Add(c.readTimeout)) // todo opt 待优化
	h, err := c.reader.NextFrame()
	if err != nil {
		return err
	}

	if h.OpCode == ws.OpClose {
		return io.EOF
	}

	*op = h.OpCode

	//if h.OpCode != ws.OpText {
	//	return fmt.Errorf("InvalidWebsocketMessage")
	//}
	// var b bytes.Buffer //opt pool

	if _, err := b.ReadFrom(c.reader); err != nil {
		return err
	}

	return nil
}

// Write writes a message.
func (c *WsConn) Write(op ws.OpCode, msg []byte) error {
	c.writer.Reset(c.conn, ws.StateClientSide, op)
	if _, err := c.writer.Write(msg); err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *WsConn) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	} else {
		return nil
	}
}

// proxyAddr: 127.0.0.1:1080
// @param excludeIpsAsPossible nil，使用文件里面的最快ip或者默认网络处理方式；非nil时，将会随机使用ip
func DialContext(ctx context.Context, url string, header ws.HandshakeHeaderHTTP, localAddr string, proxyURL string, timeout time.Duration, excludeIpsAsPossible []string) (*WsConn, error) {
	wsConn := &WsConn{}
	var dialer ws.Dialer

	dialer.Header = header

	dialer.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	if localAddr != "" {
		dialer.NetDial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			lAddr, err := net.ResolveTCPAddr("tcp", localAddr+":0")
			if err != nil {
				return nil, err
			}

			if helper.DEBUGMODE {
				log.Debug("reading ip for: ", addr)
			}
			touched := helper.TouchIpToQuery(addr)
			var rAddr *net.TCPAddr
			if touched {
				fastIp, port := helper.SelectIp(addr, excludeIpsAsPossible)
				if fastIp == "" {
					rAddr, err = net.ResolveTCPAddr("tcp", addr)
					if err != nil {
						return nil, err
					}
					wsConn.bindIp = rAddr.IP.String()
				} else {
					rAddr = &net.TCPAddr{
						IP:   net.ParseIP(fastIp),
						Port: port,
					}
					wsConn.bindIp = fastIp
					log.Infof("will use fastip %s for: %s ", fastIp, addr)
				}
			} else {
				rAddr, err = net.ResolveTCPAddr("tcp", addr)
				if err != nil {
					return nil, err
				}
				wsConn.bindIp = rAddr.IP.String()
			}
			conn, err := net.DialTCP("tcp", lAddr, rAddr)
			if err != nil {
				return nil, err
			}
			return conn, nil
		}
	} else {
		if proxyURL != "" {
			switch {
			case strings.HasPrefix(proxyURL, socks5ProxyPrefix):
				proxyAddr := proxyURL[len(socks5ProxyPrefix):]
				dialer.NetDial = SocksDialer(proxyAddr)
			}
		} else {

			dialer.NetDial = func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				if helper.DEBUGMODE {
					log.Debug("reading ip for: ", addr)
				}
				touched := helper.TouchIpToQuery(addr)
				if touched {
					fastIp, port := helper.SelectIp(addr, excludeIpsAsPossible)
					if fastIp == "" {
						var rAddr *net.TCPAddr
						rAddr, _ = net.ResolveTCPAddr("tcp", addr)
						conn, err = net.DialTCP("tcp", nil, rAddr)
						if rAddr != nil {
							wsConn.bindIp = rAddr.IP.String()
						}
					} else {
						conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", fastIp, port))
						log.Infof("will use fastip %s for: %s ", fastIp, addr)
						wsConn.bindIp = fastIp
					}
				} else {
					var rAddr *net.TCPAddr
					rAddr, _ = net.ResolveTCPAddr("tcp", addr)
					conn, err = net.DialTCP("tcp", nil, rAddr)
					if rAddr != nil {
						wsConn.bindIp = rAddr.IP.String()
					}
				}
				return
			}
		}
	}
	//conn, br, _, err := ws.Dial(ctx, url)
	dialer.Timeout = timeout // 这里必须增加一个timeout 否则url错误的情况下会一直卡在这里
	conn, _, _, err := dialer.Dial(ctx, url)
	if err != nil {
		return nil, err
	}
	//if timeout == 0 {
	//	conn.SetReadDeadline(time.Time{})
	//} else {
	//	conn.SetReadDeadline(time.Now().Add(timeout))
	//}
	wsConn.readTimeout = timeout
	wsConn.conn = conn
	wsConn.writer = wsutil.NewWriterBufferSize(conn, ws.StateClientSide, ws.OpText, wsWriteBufferSize)

	// get websocket reader
	wsConn.reader = wsutil.NewClientSideReader(conn)
	return wsConn, nil
}

func SocksDialer(proxyAddr string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	// It would be nice if we could return the error here. But we can't
	// change our API so just keep returning it in the returned Dial function.
	// Besides the implementation of proxy.SOCKS5() at the time of writing this
	// will always return nil as error.

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp", addr)
	}
	//return func(addr string) (net.Conn, error) {
	//	if err != nil {
	//		return nil, err
	//	}
	//	return dialer.Dial("tcp", addr)
	//}
}

func FormatURL(toFormat string) string {
	u, err := url.Parse(toFormat)
	if err != nil {
		return ""
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ""
	}
	addr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return ""
	}
	u.Host = net.JoinHostPort(addr.IP.String(), port)
	return u.String()
}

func OpCodeToString(op ws.OpCode) string {
	switch op {
	case ws.OpContinuation:
		return "Continuation"
	case ws.OpText:
		return "Text"
	case ws.OpBinary:
		return "Binary"
	case ws.OpClose:
		return "Close"
	case ws.OpPing:
		return "Ping"
	case ws.OpPong:
		return "Pong"
	default:
		return fmt.Sprintf("%v", op)
	}
}
