/*
// 申明私有链接维护器
privateConn local.PrivateConnector
// use private network
q.privateConn.GetLocal()
q.privateConn.TestAllPrivateConnect()
log.Warnf(q.privateConn.GetAllPrivateConnectStatus())
privateUrls := q.privateConn.GetPrivateUrls()
log.Infof("%v 可用priUrl %v", _refEx, privateUrls)
exchangeLocal := local.GetExchangeLocal(_refEx)
// 如果存在可访问的同区域的bm并且目标连接的服务器位置不是交易服务器位置

	if len(privateUrls) > 0 && q.privateConn.Local != exchangeLocal {
		// 使用私有网络
		rand.Seed(time.Now().UnixNano())
		i := rand.Intn(len(privateUrls))
		url := privateUrls[i]
		log.Infof("使用私有行情源 [%s] %s@%s", url, _refEx, refPairs[k])
		refWsPri := privateMarket.GetPrivateMarketClient(
			url,
			helper.MarketConfig{
				ExchangeName: _refEx,
				Pair:         refPairs[k],
				NeedPingPong: true,
			},
			q.refTradeMsg[k],
			refwsCb,
			privateUrls)
		refWsPri.Run()
		q.priWs = append(q.priWs, refWsPri)
	}
*/
package local

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"actor/third/log"
)

// [注意]现在改为链式连接的bm 需要访问同区域的bm获取行情 而不是访问异地的bm
type PrivateConnector struct {
	lock sync.RWMutex
	// 本机位置
	Local string
	// 本机器所在区域的bm
	canMarket []string
}

// TestAllPrivateConnect 测试私有连接 只测试本区域的连接
func (c *PrivateConnector) TestAllPrivateConnect() {
	log.Warnf("开始检测私有连接情况...")
	// 获取本机位置 能够访问的ip
	var canMarket []string
	if c.Local == LocalHK {
		canMarket = TestPrivateConnect(SourceHK)
	}
	if c.Local == LocalSG {
		canMarket = TestPrivateConnect(SourceSG)
	}
	if c.Local == LocalJP {
		canMarket = TestPrivateConnect(SourceJP)
	}
	if c.Local == LocalIE {
		canMarket = TestPrivateConnect(SourceIE)
	}
	if c.Local == LocalKR {
		canMarket = TestPrivateConnect(SourceKR)
	}
	//
	c.lock.Lock()
	defer c.lock.Unlock()
	c.canMarket = canMarket
}

// 测试私有网络能否连通
func TestPrivateConnect(privateIpString string) []string {
	can := make([]string, 0)
	if privateIpString == "" {
		return can
	}
	ips := strings.Split(privateIpString, ",")
	for _, addr := range ips {
		// client := http.Client{
		// Timeout: 300 * time.Millisecond,
		// }
		if !strings.Contains(addr, ":") {
			addr = addr + ":8080" // market 端口
		}
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err != nil {
			log.Warnf("%v 专线访问失败 %v", addr, err)
		} else {
			if conn != nil {
				_ = conn.Close()
				can = append(can, addr)
			} else {
				log.Warnf("failed to connect special line")
			}
		}

		// r, e := client.Get(fmt.Sprintf("http://%v:8888/status", addr))
		// if e != nil {
		// 	log.Warnf("%v 专线访问失败 %v", addr, e)
		// 	continue
		// }
		// defer r.Body.Close()
		// if r.StatusCode == 200 {
		// 	log.Infof("%v 专线访问成功", addr)
		// 	can = append(can, addr)
		// }

	}
	return can
}

// GetLocal 获取本机器位置
func (c *PrivateConnector) GetLocal() {
	c.Local = GetLocal()
}

// GetAllPrivateConnectStatus 获取私有连接信息
func (c *PrivateConnector) GetAllPrivateConnectStatus() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	msg := fmt.Sprintf(" canMarket:%v", c.canMarket)
	return msg
}

// GetPrivateUrls 获取私有连接的ip地址
func (c *PrivateConnector) GetPrivateUrls() []string {
	c.lock.RLock()
	canMarket := c.canMarket
	c.lock.RUnlock()
	// beastmarket改为链式连接 行情服务只获取本地区域的行情
	return canMarket
}
