// @title Xiaozhi Server API Documentation
// @version 1.0
// @description Xiaozhi Server API including OTA, Vision, and Administration endpoints
// @host localhost:8080
// @BasePath /api
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/auth/store"
	"xiaozhi-server-go/src/core/provider"
	"xiaozhi-server-go/src/core/transport"
	"xiaozhi-server-go/src/core/transport/websocket"
	"xiaozhi-server-go/src/core/utils"
	_ "xiaozhi-server-go/src/docs"
	"xiaozhi-server-go/src/httpsvr/ota"
	"xiaozhi-server-go/src/httpsvr/vision"

	cfg "xiaozhi-server-go/src/httpsvr/webapi"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Import providers to register init functions
	_ "xiaozhi-server-go/src/core/providers/asr/deepgram"
	_ "xiaozhi-server-go/src/core/providers/asr/doubao"
	_ "xiaozhi-server-go/src/core/providers/asr/gosherpa"
	_ "xiaozhi-server-go/src/core/providers/asr/iflytek"
	_ "xiaozhi-server-go/src/core/providers/asr/stepfun"
	_ "xiaozhi-server-go/src/core/providers/llm/coze"
	_ "xiaozhi-server-go/src/core/providers/llm/doubao"
	_ "xiaozhi-server-go/src/core/providers/llm/dsh"
	_ "xiaozhi-server-go/src/core/providers/llm/ollama"
	_ "xiaozhi-server-go/src/core/providers/llm/openai"
	_ "xiaozhi-server-go/src/core/providers/tts/deepgram"
	_ "xiaozhi-server-go/src/core/providers/tts/doubao"
	_ "xiaozhi-server-go/src/core/providers/tts/edge"
	_ "xiaozhi-server-go/src/core/providers/tts/gosherpa"
	_ "xiaozhi-server-go/src/core/providers/tts/iflytek"
	_ "xiaozhi-server-go/src/core/providers/vlllm/ollama"
	_ "xiaozhi-server-go/src/core/providers/vlllm/openai"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

func LoadConfigAndLogger() (*configs.Config, *utils.Logger, error) {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found, using system environment variables")
	}

	// Initialize database connection
	_, _, err = database.InitDB()
	if err != nil {
		fmt.Printf("Database connection failed: %v\n", err)
	}
	// Load configuration, defaults to .config.yaml
	config, configPath, err := configs.LoadConfig(database.GetServerConfigDB())
	if err != nil {
		return nil, nil, err
	}

	// Initialize logger
	logger, err := utils.NewLogger((*utils.LogCfg)(&config.Log))
	if err != nil {
		return nil, nil, err
	}
	logger.Info("[Log] [Initialized %s] successfully", configPath)
	utils.DefaultLogger = logger
	logger.Info("Logger initialized successfully, level: %s, config path: %s", config.Log.LogLevel, configPath)

	database.SetLogger(logger)
	database.InsertDefaultConfigIfNeeded(database.GetDB())

	return config, logger, nil
}

// initAuthManager initializes authentication manager
func initAuthManager(config *configs.Config, logger *utils.Logger) (*auth.AuthManager, error) {
	storeConfig := &store.StoreConfig{
		Type:     config.Server.Auth.Store.Type,
		ExpiryHr: config.Server.Auth.Store.Expiry,
		Config:   make(map[string]interface{}),
	}

	authManager, err := auth.NewAuthManager(storeConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth manager: %v", err)
	}

	return authManager, nil
}

func StartTransportServer(
	config *configs.Config,
	logger *utils.Logger,
	authManager *auth.AuthManager,
	g *errgroup.Group,
	groupCtx context.Context,
) (*transport.TransportManager, error) {
	providerManager, err := provider.NewProviderManager(config, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to initialize provider manager: %v", err))
		return nil, fmt.Errorf("failed to initialize provider manager: %v", err)
	}

	transportManager := transport.NewTransportManager(config, logger)

	handlerFactory := transport.NewDefaultConnectionHandlerFactory(
		config,
		providerManager,
		logger,
	)

	enabledTransports := make([]string, 0)

	// Check WebSocket transport configuration
	if config.Transport.WebSocket.Enabled {
		wsTransport := websocket.NewWebSocketTransport(config, logger)
		wsTransport.SetConnectionHandler(handlerFactory)
		transportManager.RegisterTransport("websocket", wsTransport)
		enabledTransports = append(enabledTransports, "WebSocket")
		logger.Debug("WebSocket transport registered")
	}

	if len(enabledTransports) == 0 {
		return nil, fmt.Errorf("no transport enabled")
	}

	logger.Info("[Transport] [Enabled %v]", enabledTransports)

	// Start transport services
	g.Go(func() error {
		// Listen for shutdown signal
		go func() {
			<-groupCtx.Done()
			logger.Info("Received shutdown signal, stopping all transports...")
			if err := transportManager.StopAll(); err != nil {
				logger.Error("Failed to stop transports: %v", err)
			} else {
				logger.Info("All transports stopped gracefully")
			}
		}()

		if err := transportManager.StartAll(groupCtx); err != nil {
			if groupCtx.Err() != nil {
				return nil // Graceful exit
			}
			logger.Error("Transport execution failed: %v", err)
			return err
		}
		return nil
	})

	logger.Debug("Transport service started successfully")
	return transportManager, nil
}

func StartHttpServer(
	config *configs.Config,
	logger *utils.Logger,
	g *errgroup.Group,
	groupCtx context.Context,
) (*http.Server, error) {
	if config.Log.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	router.SetTrustedProxies([]string{"0.0.0.0"})

	// Configure global CORS middleware
	corsConfig := cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"Cache-Control",
			"X-File-Name",
			"client-id",
			"device-id",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	logger.Debug("Global CORS middleware configured, supports OPTIONS preflight")

	// API routes mounted under /api
	apiGroup := router.Group("/api")

	// Static assets from ./web
	router.Use(static.Serve("/", static.LocalFile("./web", true)))

	// History fallback routing
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(404, gin.H{"error": "api not found"})
			return
		}

		c.File("./web/index.html")
	})

	// Start OTA service
	otaService := ota.NewDefaultOTAService(config.Web.Websocket)
	if err := otaService.Start(groupCtx, router, apiGroup); err != nil {
		logger.Error("Failed to start OTA service: %v", err)
		return nil, err
	}

	// Start Vision service
	visionService, err := vision.NewDefaultVisionService(config, logger)
	if err != nil {
		logger.Error("Failed to initialize Vision service: %v", err)
		return nil, err
	}
	if err := visionService.Start(groupCtx, router, apiGroup); err != nil {
		logger.Error("Failed to start Vision service: %v", err)
		return nil, err
	}

	cfgServer, err := cfg.NewDefaultAdminService(config, logger)
	if err != nil {
		logger.Error("Failed to initialize Admin service: %v", err)
		return nil, err
	}
	if err := cfgServer.Start(groupCtx, router, apiGroup); err != nil {
		logger.Error("Failed to start Admin service: %v", err)
		return nil, err
	}

	userServer, err := cfg.NewDefaultUserService(config, logger)
	if err != nil {
		logger.Error("Failed to initialize User service: %v", err)
		return nil, err
	}
	if err := userServer.Start(groupCtx, router, apiGroup); err != nil {
		logger.Error("Failed to start User service: %v", err)
		return nil, err
	}

	// Start system config service
	systemConfigService := cfg.NewSystemConfigService(logger, database.GetDB())
	systemConfigService.RegisterRoutes(apiGroup)

	// HTTP Server with graceful shutdown
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(config.Web.Port),
		Handler: router,
	}

	// Register Swagger API docs
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	g.Go(func() error {
		logger.Info("Gin HTTP server started, listening at: http://localhost:%d", config.Web.Port)

		go func() {
			<-groupCtx.Done()

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("Failed to shut down HTTP server: %v", err)
			} else {
				logger.Info("HTTP server shut down gracefully")
			}
		}()

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed to start: %v", err)
			return err
		}
		return nil
	})

	return httpServer, nil
}

func GracefulShutdown(cancel context.CancelFunc, logger *utils.Logger, g *errgroup.Group) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	sig := <-sigChan
	logger.Info("Received system signal: %v, starting graceful shutdown", sig)

	cancel()

	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error("Error occurred during shutdown: %v", err)
			os.Exit(1)
		}
		logger.Info("All services shut down gracefully")
	case <-time.After(15 * time.Second):
		logger.Error("Shutdown timeout exceeded, exiting forcibly")
		os.Exit(1)
	}
}

func startServices(
	config *configs.Config,
	logger *utils.Logger,
	authManager *auth.AuthManager,
	g *errgroup.Group,
	groupCtx context.Context,
) error {
	if _, err := StartTransportServer(config, logger, authManager, g, groupCtx); err != nil {
		return fmt.Errorf("failed to start transport service: %w", err)
	}

	if _, err := StartHttpServer(config, logger, g, groupCtx); err != nil {
		return fmt.Errorf("failed to start HTTP service: %w", err)
	}

	return nil
}

func main() {
	config, logger, err := LoadConfigAndLogger()
	if err != nil {
		fmt.Println("Failed to load config or initialize logger:", err)
		os.Exit(1)
	}

	authManager, err := initAuthManager(config, logger)
	if err != nil {
		logger.Error("Failed to initialize auth manager: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, groupCtx := errgroup.WithContext(ctx)

	if err := startServices(config, logger, authManager, g, groupCtx); err != nil {
		logger.Error("Failed to start services: %v", err)
		cancel()
		os.Exit(1)
	}

	GracefulShutdown(cancel, logger, g)

	if authManager != nil {
		authManager.Close()
	}

	logger.Info("Application exited successfully")
	logger.Close()
}
