//go:build prd
// +build prd

package config

import (
	"fmt"

	bt "actor/tools"
)

const ENV = "prd"

// var REDIS_ADDR = "172.17.15.225:6379"

func init() {

	// if config.ENV == "prd" {
	area := bt.MustGetServerArea()
	fmt.Println("area " + area)
	if area == bt.AreaSG {
		// REDIS_ADDR = "172.18.235.126:6379" // aws 实例
		//REDIS_ADDR = "172.18.23.95:8379" // sg自搭redis
		//REDIS_PWD = "qq666love"          // 自搭redis
	} else if area == bt.AreaHK {
		REDIS_ADDR = "r-j6chl6g86pk54jtyne.redis.rds.aliyuncs.com:6379" // ali 实例
	}
	// }
}
