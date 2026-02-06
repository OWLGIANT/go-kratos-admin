package main

import (
	"actor/schedule"
	"context"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/signal"
	"syscall"

	"actor/server/backend"
	"actor/server/config"
	"actor/server/service"
	"actor/third/log"
)

var (
	configPath   = flag.String("c", "configs/config.yaml", "config file path")
	backendURL   = flag.String("backend", "", "backend websocket url (overrides env BACKEND_WS_URL)")
	backendToken = flag.String("token", "", "backend auth token (overrides env BACKEND_WS_TOKEN)")
	version      = "1.0.0"
	buildTime    = "unknown"
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log.Init(cfg.Log.File, cfg.Log.Level)
	logger := log.RootLogger
	logger.Infof("Actor server starting, version: %s, build: %s", version, buildTime)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create robot manager
	robotManager := service.NewRobotManager(ctx, logger, buildTime)

	// Create exchange manager (for backward compatibility)
	exchangeManager := service.NewExchangeManager(logger)

	// 初始化定时任务调度器
	// 会立即执行一次 IP池生成，然后每2小时自动执行一次
	schedule.InitSchedule()

	// Initialize exchanges from config
	for _, exCfg := range cfg.Exchanges {
		brokerCfg := exCfg.ToBrokerConfig()
		brokerCfg.Logger = logger
		brokerCfg.RootLogger = logger

		if err := exchangeManager.InitExchange(exCfg.Name, brokerCfg); err != nil {
			logger.Errorf("Failed to initialize exchange %s: %v", exCfg.Name, err)
		} else {
			logger.Infof("Exchange %s initialized", exCfg.Name)
		}
	}

	// Setup backend connection if configured
	var backendClient *backend.Client
	wsURL := getEnvOrFlag(*backendURL, "BACKEND_WS_URL")
	wsToken := getEnvOrFlag(*backendToken, "BACKEND_WS_TOKEN")
	log.Infof("========Backend URL=====: %s", wsURL)
	log.Infof("========Backend Token=====: %s", wsToken)
	if wsURL != "" {
		robotID := fmt.Sprintf("actor-%s", uuid.NewString())
		exchange := "multi"
		tenantID := uint32(0)

		if len(cfg.Exchanges) > 0 {
			robotID = cfg.Exchanges[0].RobotID
			exchange = cfg.Exchanges[0].Name
		}

		// Create command handler with robot manager
		handler := backend.NewDefaultHandler().
			OnCreate(func(data map[string]interface{}) error {
				robotID, _ = data["robot_id"].(string)
				robotType, _ := data["robot_type"].(string)
				if robotType == "" {
					robotType = "cat"
				}
				logger.Infof("Received create command: robot_id=%s, type=%s", robotID, robotType)
				if err = robotManager.CreateRobot(robotID, robotType, data); err != nil {
					return err
				}
				return robotManager.StartRobot(robotID, data)
			}).
			OnStart(func(data map[string]interface{}) error {
				robotID, _ = data["robot_id"].(string)
				logger.Infof("Received start command: robot_id=%s", robotID)
				return robotManager.StartRobot(robotID, data)
			}).
			OnStop(func(data map[string]interface{}) error {
				robotID, _ = data["robot_id"].(string)
				logger.Infof("Received stop command: robot_id=%s", robotID)
				return robotManager.StopRobot(robotID, data)
			}).
			OnStatus(func() (interface{}, error) {
				logger.Info("Received status query")
				return robotManager.GetAllRobots(), nil
			}).
			OnConfig(func(data map[string]interface{}) error {
				logger.Info("Received config update")
				return nil
			}).
			OnDelete(func(data map[string]interface{}) error {
				robotID, _ = data["robot_id"].(string)
				logger.Infof("Received delete command: robot_id=%s", robotID)
				return robotManager.DeleteRobot(robotID)
			})

		// Create backend client
		backendClient = backend.NewClient(backend.ClientConfig{
			URL:      wsURL,
			Token:    wsToken,
			RobotID:  robotID,
			Exchange: exchange,
			Version:  version,
			TenantID: tenantID,
		}, handler, logger)

		backendClient.OnConnect(func() {
			logger.Info("Connected to backend......")

			// Send server info after connection
			// 必须字段: nickname, port (长度限制在 CollectServerSyncData 中处理)
			nickname := cfg.Server.Nickname
			if nickname == "" {
				nickname = robotID // 默认使用 robotID
			}
			port := fmt.Sprintf("%d", cfg.Server.HTTPPort)
			serverData := backend.CollectServerSyncData(robotID, nickname, port)
			if err = backendClient.SendServerSync(serverData); err != nil {
				logger.Errorf("Failed to send server sync: %v", err)
			} else {
				logger.Infof("Server sync sent: ip=%s, inner_ip=%s, port=%s, nickname=%s, machine_id=%s",
					serverData.IP, serverData.InnerIP, serverData.Port, serverData.Nickname, serverData.MachineID)
			}
		})

		backendClient.OnDisconnect(func(err error) {
			logger.Warnf("Disconnected from backend: %v", err)
		})

		// Set backend client to robot manager
		robotManager.SetBackendClient(backendClient)

		// Connect to backend
		if err = backendClient.Connect(); err != nil {
			logger.Errorf("Failed to connect to backend: %v", err)
		} else {
			backendClient.Run()
			logger.Infof("====Connected to backend===========")
		}
	}

	logger.Info("Actor server started successfully")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down actor server...")

	// Cleanup
	if backendClient != nil {
		backendClient.Close()
	}
	robotManager.StopAll()
	exchangeManager.Stop()

	logger.Info("Actor server stopped")
}

// getEnvOrFlag returns flag value if set, otherwise returns env value
func getEnvOrFlag(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}
