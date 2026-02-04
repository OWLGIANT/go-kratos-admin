package helper

import (
	"fmt"
	"os"
	"testing"
)

func TestIpPool(t *testing.T) {
	ipPoolTrue := make(map[string]string)
	// ipPoolTrue["172.31.47.169"] = "57.181.4.108"
	ips, _ := GetClientIp()
	fmt.Println("client ips", ips)

	publicIps, pool := GetIpPoolfromFile(ips)
	fmt.Println("publicIps", publicIps)
	fmt.Println("pool", pool)

	os.Remove("ipPool_v1.json")

	publicIps, pool = GetIpPoolfromFile(ips)
	fmt.Println("publicIps from provider", publicIps)
	fmt.Println("pool from provider", pool)

	//----------------------------------------------------------------
	providers := testWebsites(t, ipPoolTrue, []string{
		"http://ident.me", "https://ipv4.netarm.com",
		"http://api.ip.sb/ip", "http://api.ipify.org/",
		"https://api-bdc.net/data/client-ip", "https://api.seeip.org",
		"http://checkip.dyndns.org", "https://ipinfo.io/ip",
		"https://freeipapi.com/api/json/", "https://api.ipapi.is/"})

	fmt.Println(providers)
}

func testWebsites(t *testing.T, ipPoolTrue map[string]string, providers []string) (goodProviders []string) {

	for _, provider := range providers {
		// make sure to remove the ipPool_v1.json before testing a new website
		if _, err := os.Stat("ipPool_v1.json"); err == nil {
			err := os.Remove("ipPool_v1.json")
			if err != nil {
				fmt.Println("Error removing file:", err)
			} else {
				fmt.Println("File removed successfully.")
			}
		}

		privateIps := make([]string, 0, len(ipPoolTrue))
		for key := range ipPoolTrue {
			privateIps = append(privateIps, key)
		}
		_, ipPoolResult := GetIpPoolfromProviders(privateIps,
			[]string{provider})

		passed := true

		for k, v := range ipPoolTrue {
			val, got := ipPoolResult[k]
			fmt.Printf("Priv %s Pub %s\n", k, val)
			passed = passed && got && val == v

			// CheckPoint(t, true == got, "Private Ip in ipPool "+k)
			// CheckPoint(t, v == val, "Public Ip is: "+val+" for the private Ip: "+k)
		}

		// CheckPoint(t, passed, provider)
		goodProviders = append(goodProviders, provider)
	}

	return goodProviders
}
