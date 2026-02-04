package main

import (
	"flag"
	"fmt"
	"os"

	"actor/helper"
	"actor/third/log"
)

func main() {
	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息")
	flag.Parse()

	if showHelp {
		fmt.Println("IP池生成工具")
		fmt.Println("用法: ippool")
		fmt.Println("功能: 获取本机所有私有IP，查询对应的公网IP，并保存到 ipPool_v1.json 文件")
		os.Exit(0)
	}

	log.Info("开始生成 IP池...")

	// 获取本机私有IP
	privateIps, err := helper.GetClientIp()
	if err != nil {
		log.Errorf("获取本机IP失败: %s", err.Error())
		os.Exit(1)
	}
	log.Infof("本机私有IP: %v", privateIps)

	// 生成IP池
	if err := helper.GenIpPool(); err != nil {
		log.Errorf("生成IP池失败: %s", err.Error())
		os.Exit(1)
	}

	log.Info("IP池生成完成，已保存到 ipPool_v1.json")
}
