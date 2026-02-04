// // 策略启动脚本
package main

//
//import (
//	"beasthunter/config"
//	"context"
//	"flag"
//	"fmt"
//	"actor/broker/brokerconfig"
//	"actor/limit"
//	"actor/third/log"
//	"github.com/shirou/gopsutil/v3/mem"
//	"os"
//	"os/signal"
//	"runtime"
//	"strings"
//	"syscall"
//	"time"
//)
//
//// 全局变量 在init之前被执行
//var (
//	applicationName string
//	buildTime       string // 编译时间
//	gitCommitHash   string // git提交hash
//	gitTag          string // git标签
//	goVersion       string // go编译器版本
//	ipLimitation    = ""   // ip限制表
//)
//
//var (
//	configFile string // 配置文件名字
//	help       bool   // 显示帮助
//	ver        bool   // 显示版本
//	num        int    // 进程号
//	logFile    string // 日志文件名
//	dbg        bool   // 是否为debug模式启动
//	exit       bool   // 是否为退出任务
//	msg        string // 需要传递的启动信息 当执行退出任务时 这个msg包含退出原因
//)
//
//// 在包初始化后自动执行 并且在main之前执行 无法被显示调用
//func init() {
//	runtime.GOMAXPROCS(runtime.NumCPU()) // 使用所有cpu运行此程序 确保最大性能
//}
//
//// 入口函数
//func main() {
//	v, _ := mem.VirtualMemory()
//	// almost every return value is a struct
//	fmt.Printf("Total: %v, Free:%v, UsedPercent:%f%%\n", v.Total, v.Free, v.UsedPercent)
//

//
//	//
//	fmt.Println("入参:", os.Args)
//	// 获取启动参数
//	flag.StringVar(&configFile, "c", "", "set configuration `toml file`")
//	flag.BoolVar(&help, "h", false, "this help")
//	flag.BoolVar(&dbg, "d", false, "debug mode")
//	flag.BoolVar(&ver, "v", false, "version")
//	flag.BoolVar(&exit, "exit", false, "exit")
//	flag.IntVar(&num, "num", 0, "")
//	flag.StringVar(&logFile, "log_file", "", "log file")
//	flag.StringVar(&msg, "msg", "", "msg")
//	flag.Parse()
//
//	// 帮助信息
//	if help {
//		fmt.Printf("show me the money")
//		return
//	}
//
//	// 强制要求配置文件
//	if len(configFile) == 0 || !strings.Contains(configFile, ".toml") {
//		fmt.Printf("not input toml file")
//		configFile = "config.toml"
//	}
//
//	// 打印版本信息
//	if ver {
//		fmt.Println(applicationName)
//		fmt.Printf("Build Time: %s\n", buildTime)
//		fmt.Printf("git Commit Hash: %s\n", gitCommitHash)
//		fmt.Printf("git Tag: %s\n", gitTag)
//		fmt.Printf("Go Version: %s\n", goVersion)
//		return
//	}
//
//	log.Info(applicationName)
//	log.Infof("Build Time: %s\n", buildTime)
//	log.Infof("git Commit Hash: %s\n", gitCommitHash)
//	log.Infof("git Tag: %s\n", gitTag)
//	log.Infof("Go Version: %s\n", goVersion)
//
//	// 限制非法启动
//	if len(ipLimitation) > 0 {
//		fmt.Printf("Only for IP:%s\n", ipLimitation)
//	} else {
//		fmt.Println("No limit")
//	}
//	limit.VerifyLimitation(ipLimitation)
//
//	// 获取进程号
//	pid := os.Getpid()
//	fmt.Printf("当前进程: %d\n", pid)
//
//	if !exit {
//		// 清仓程序不要设置优先级
//		// 设置本进程的优先级
//		err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, -10)
//		if err != nil {
//			fmt.Println("优先级设置失败:", err.Error())
//			//return
//		}
//	}
//	// 加载base配置文件
//	beastCfg := brokerconfig.LoadBaseConfig("beast.toml")
//	fmt.Println("Beast Config: ", beastCfg)
//
//	// 加载配置文件
//	cfg := config.LoadConfig(configFile, !dbg)
//	switch cfg.LogLevel {
//	case log.DebugLevel, log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.PanicLevel:
//		if logFile == "" {
//			logFile = "test.log"
//		}
//
//		if dbg || cfg.Proxy != "" {
//			log.Init(logFile, cfg.LogLevel,
//				log.SetStdout(true),
//				log.SetCaller(true),
//				log.SetMaxBackups(1),
//			)
//		} else {
//			log.Init(logFile, cfg.LogLevel,
//				log.SetSLog(true),
//				log.SetMaxBackups(1),
//			)
//		}
//
//	default:
//		_cfg := *cfg
//		_cfg.AccessKey = "xxx"
//		_cfg.SecretKey = "xxx"
//		fmt.Printf("%#v\n", _cfg)
//		log.Init("debug.log", log.DebugLevel,
//			log.SetSLog(true),
//			log.SetMaxBackups(1),
//		)
//	}
//
//	fmt.Printf("日志等级:%s \n", cfg.LogLevel)
//	// 获取上下文
//	ctx, cancel := context.WithCancel(context.Background())
//
//	// 启动协程 监听停机信号
//	go func() {
//		exitSignal := make(chan os.Signal, 1)
//		sigs := []os.Signal{os.Interrupt, syscall.SIGILL, syscall.SIGINT, syscall.SIGKILL, syscall.SIGQUIT, syscall.SIGTERM}
//		signal.Notify(exitSignal, sigs...)
//
//		select {
//		case sig := <-exitSignal:
//			// 关闭上下文
//			cancel()
//			log.Info("exit - ", sig)
//			fmt.Println("exit - ", sig)
//			return
//		case <-ctx.Done():
//			cancel()
//			log.Warnf("runtime退出")
//			fmt.Println("runtime退出")
//			return
//		}
//	}()
//
//	// todo 震哥在这里实例化策略 然后run
//
//	// todo  run 必须阻塞主进程在这里 如果需要退出了 再return到主进程这里
//
//	log.Errorf("退出进程[%d]", pid)
//
//	// 写入日志 为了性能是异步写入 再退出时必须主动调用一次才能确保所有日志全部输出 运行期间不用主动调用会间歇式写入
//	log.Sync()
//	time.Sleep(time.Second)
//}
