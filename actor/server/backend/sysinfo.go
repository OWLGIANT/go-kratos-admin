package backend

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// CollectServerInfo 收集服务器信息
func CollectServerInfo() *ServerInfo {
	info := &ServerInfo{}

	// CPU 信息
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		info.CPU = fmt.Sprintf("%.1f%%", cpuPercent[0])
	}

	// 内存信息
	if memInfo, err := mem.VirtualMemory(); err == nil {
		info.Mem = float64(memInfo.Used) / 1024 / 1024 / 1024 // GB
		info.MemPct = fmt.Sprintf("%.1f%%", memInfo.UsedPercent)
	}

	// 磁盘信息
	if diskInfo, err := disk.Usage("/"); err == nil {
		info.DiskPct = fmt.Sprintf("%.1f%%", diskInfo.UsedPercent)
	}

	return info
}

// CollectServerSyncData 收集完整的服务器同步数据
func CollectServerSyncData(robotID string) *ServerSyncData {
	data := &ServerSyncData{
		RobotID:    robotID,
		ServerInfo: CollectServerInfo(),
	}

	// IP 地址
	ips := getLocalIPs()
	for _, ip := range ips {
		if isPrivateIP(ip) {
			data.InnerIP = ip
		} else {
			data.IP = ip
		}
	}

	// 如果没有外网 IP，尝试获取出站 IP
	if data.IP == "" {
		data.IP = GetOutboundIP()
	}

	// 机器ID
	data.MachineID = GetMachineID()

	return data
}

// getLocalIPs 获取本机 IP 地址
func getLocalIPs() []string {
	var ips []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// 跳过 down 的接口和 loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只要 IPv4
			ip = ip.To4()
			if ip == nil {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	return ips
}

// isPrivateIP 判断是否为私有 IP
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 私有 IP 范围
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// GetGoroutineCount 获取当前 goroutine 数量
func GetGoroutineCount() int {
	return runtime.NumGoroutine()
}

// GetOutboundIP 获取出站 IP（用于获取外网 IP）
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetMachineID 获取机器唯一标识
func GetMachineID() string {
	// 1. 尝试从 /etc/machine-id 读取 (Linux)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		return string(data[:len(data)-1]) // 去掉换行符
	}

	// 2. 尝试从 /var/lib/dbus/machine-id 读取 (Linux)
	if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
		return string(data[:len(data)-1])
	}

	// 3. 使用 hostname
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return ""
}
