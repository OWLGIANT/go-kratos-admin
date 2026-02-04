package limit

import (
	"fmt"
	"net"
	"os"
)

func GetMyIP() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops: " + err.Error() + "\n")
		//os.Exit(1)
		return nil
	}

	ips := make([]string, 0)

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

func VerifyLimitation(ipLimitation string) {
	if len(ipLimitation) == 0 {
		return
	}
	fmt.Println("ipl:", ipLimitation)
	ips := GetMyIP()
	if len(ips) == 0 {
		panic("no ip")
	}

	found := false
	for _, v := range ips {
		if v == ipLimitation {
			found = true
			break
		}
	}
	if !found {
		panic("not support this vps")
	}
}
