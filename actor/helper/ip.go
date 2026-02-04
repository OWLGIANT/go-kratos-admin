// 本机器ip地址查询和管理相关工具函数
package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"actor/third/log"
)

/* ---------------------------------------------------------------------------------------------------------------- */

// 获取本机器ip 私有
func GetClientIp() ([]string, error) {
	var ips []string
	ipsMap := make(map[string]string)

	addrs, err := net.InterfaceAddrs()

	for i := 0; i < 5; i++ {
		if err != nil {
			addrs, err = net.InterfaceAddrs()
		} else {
			break
		}
	}
	if err != nil {
		log.Errorf("获取本机ip失败 %s", err.Error())
		return []string{}, err
	}

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				//fmt.Println(ip)
				if !strings.Contains(ip, "127.0.0.1") {
					ipsMap[ip] = ""
				}
			}

		}
	}
	for ip := range ipsMap {
		ips = append(ips, ip)
	}
	//fmt.Println(ips)
	//fmt.Println(len(ips))
	if len(ips) >= 1 {
		return ips, nil
	} else {
		return []string{}, fmt.Errorf("获取本机ip失败")
	}
}

// HttpGetFromIP 获取外网ip
func HttpGetFromIP(url, ipaddr string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				//本地地址  ipaddr是本地外网IP
				lAddr, err := net.ResolveTCPAddr(netw, ipaddr+":0")
				if err != nil {
					return nil, err
				}
				//被请求的地址
				rAddr, err := net.ResolveTCPAddr(netw, addr)
				if err != nil {
					return nil, err
				}
				conn, err := net.DialTCP(netw, lAddr, rAddr)
				if err != nil {
					return nil, err
				}
				deadline := time.Now().Add(35 * time.Second)
				conn.SetDeadline(deadline)
				return conn, nil
			},
			DisableKeepAlives: true,
		},
		Timeout: time.Second,
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/27.0.1453.93 Safari/537.36")
	return client.Do(req)
}

/* ---------------------------------------------------------------------------------------------------------------- */

// GetIpPool 获取私有ip对应的公网ip
// 云服务器运营商层面 是允许 一个公网ip对应多个私有ip 但是bq层强制必须11对应
func GetIpPool(privateIps []string) ([]string, map[string]string) {
	return GetIpPoolfromFile(privateIps)
}

var PROVIDERS = []string{
	"http://ident.me", "https://ipv4.netarm.com",
	"http://api.ip.sb/ip", "http://api.ipify.org/",
	"https://api-bdc.net/data/client-ip", "https://api.seeip.org",
	"http://checkip.dyndns.org", "https://ipinfo.io/ip",
	"https://freeipapi.com/api/json/", "https://api.ipapi.is/"}

func GetIpPoolfromFile(privateIps []string) ([]string, map[string]string) {
	var publicIps []string
	ipPool := make(map[string]string)
	fileName := "ipPool_v1.json"
	data, err := os.ReadFile(fileName)
	if err != nil {
		log.Errorf("GetIpPoolfromFile os.ReadFile err: %s", err.Error())
		return GetIpPoolfromProviders(privateIps, PROVIDERS)
	}
	if err = json.Unmarshal(data, &ipPool); err != nil {
		log.Errorf("GetIpPoolfromFile json.Unmarshal err: %s", err.Error())
		return GetIpPoolfromProviders(privateIps, PROVIDERS)
	}
	for _, pri := range privateIps {
		if val, ok := ipPool[pri]; ok {
			publicIps = append(publicIps, val)
		} else {
			//有一个找不到就从网络更新
			return GetIpPoolfromProviders(privateIps, PROVIDERS)
		}
	}
	return publicIps, ipPool
}

//	GetIpPoolfromProviders(privateIps, []string{
//		"http://ident.me", "https://ipv4.netarm.com",
//		"http://api.ip.sb/ip", "http://api.ipify.org/",
//		"https://api-bdc.net/data/client-ip", "https://api.seeip.org",
//		"http://checkip.dyndns.org", "https://ipinfo.io/ip",
//		"https://freeipapi.com/api/json/", "https://api.ipapi.is/", "http://server.thousandquant.com:8500/myip",
//		"http://auth.thousandquant.com:8500/myip"})
func GetIpPoolfromProviders(privateIps []string, providers []string) ([]string, map[string]string) {
	if len(privateIps) == 0 {
		log.Warnf("没有传入私有ip")
		return make([]string, 0), make(map[string]string)
	}
	// 尝试从文件中读取 IpPool
	// 通过文件名来明确版本号
	fileName := "ipPool_v1.json"
	modTs := GetFileModTime(fileName)
	if time.Now().Unix()-modTs > 300 {
		log.Warnf("需要更新 IpPool")
	} else {
		log.Infof("不需要更新 IpPool 从文件加载...")
		byteDate, err := os.ReadFile(fileName)
		if err != nil {
			log.Infof("读取文件失败 需要更新 IpPool")
		} else {
			ipPool := make(map[string]string)
			var publicIps []string
			err := json.Unmarshal(byteDate, &ipPool)
			if err != nil {
				log.Errorf("解析文件失败 需要更新 IpPool")
			} else {
				for privateIp, publicIp := range ipPool { // key 为私有ip value为公网ip
					for _, ip := range privateIps {
						if ip == privateIp { // 按照privateIp顺序返回publicIp
							publicIps = append(publicIps, publicIp)
						}
					}
				}
				if len(publicIps) != len(privateIps) {
					log.Warnf("文件中的IpPool存在异常 %v!=%v", len(publicIps), len(privateIps))
				}
				if len(publicIps) == 0 {
					log.Warnf("文件中的IpPool为空")
				}
				// 一切正常
				if len(publicIps) == len(privateIps) && len(publicIps) > 0 {
					return publicIps, ipPool
				}
			}
		}
	}

	/* ------------------------------------------------------------------------------------------ */
	ipPool, publicIps, _ := getIp(privateIps, providers)
	/* ------------------------------ */
	//  保存到文件
	jsonData, _ := json.Marshal(ipPool)
	os.WriteFile(fileName, jsonData, 0644)
	return publicIps, ipPool
}

/* ---------------------------------------------------------------------------------------------------------------- */
func getIp(privateIps []string, providers []string) (map[string]string, []string, []string) {
	ipPool := make(map[string]string)
	var failIps []string
	var publicIps []string

	for _, ip := range privateIps {
		time.Sleep(time.Millisecond * 100)
		for idx, provider := range providers {

			r, e := HttpGetFromIP(provider, ip)
			if e != nil {
				if idx == len(providers)-1 {
					log.Warnf("%v 查询公网IP失败", ip)
					failIps = append(failIps, ip)
				}
				continue
			}
			if r.StatusCode != 200 {
				if idx == len(providers)-1 {
					log.Warnf("%v 查询公网IP失败", ip)
					failIps = append(failIps, ip)
				}
				r.Body.Close()
				continue
			}
			body, _ := io.ReadAll(r.Body)
			publicIp := regexp.MustCompile(`(?:\d{1,3}\.)+(?:\d{1,3})`).FindString(BytesToString(body))
			publicIps = append(publicIps, publicIp)
			ipPool[ip] = publicIp
			log.Warnf("公网:%v 私有:%v", publicIp, ip)
			r.Body.Close()
			break
		}
	}

	return ipPool, publicIps, failIps
}

/* ---------------------------------------------------------------------------------------------------------------- */

// GenIpPool 主动生成IP池文件
// 获取本机所有私有IP，并查询对应的公网IP，保存到 ipPool_v1.json 文件
func GenIpPool() error {
	privateIps, err := GetClientIp()
	if err != nil {
		log.Errorf("GenIpPool GetClientIp err: %s", err.Error())
		return err
	}

	publicIps, ipPool := GetIpPoolfromProviders(privateIps, PROVIDERS)
	log.Infof("GenIpPool 完成 - 私有IP: %v, 公网IP: %v, IP池: %v", privateIps, publicIps, ipPool)
	return nil
}
