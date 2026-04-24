package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"vrec/internal/config"
	"vrec/internal/handler"
	"vrec/internal/middleware"
	"vrec/internal/repository"
	"vrec/internal/service"
	"vrec/pkg/logger"
)

var (
	path = "config.toml"
)

func parseCommand() {
	flag.StringVar(&path, "path", path, "server config path")
	flag.Parse()
}

func main() {
	parseCommand()

	// 加载配置
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 初始化日志
	zapLog, err := logger.NewLogger(&cfg.Logger)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer zapLog.Sync()

	// 初始化数据库
	db, err := repository.NewDB(&cfg.DB)
	if err != nil {
		zapLog.Fatal("failed to connect database", zap.Error(err))
	}
	defer db.Close()

	// 初始化 Repository
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewUserTokenRepository(db)
	rechargeRepo := repository.NewUserRechargeRepository(db)
	rechargeOrderRepo := repository.NewRechargeOrderRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	resultRepo := repository.NewTranscriptionResultRepository(db)
	summaryRepo := repository.NewMeetingSummaryRepository(db)

	// 初始化 Service
	userService := service.NewUserService(userRepo, tokenRepo, rechargeRepo, zapLog)
	s3Service := service.NewS3Service(cfg, zapLog)
	asrService := service.NewASRService(&cfg.ASR, zapLog)
	llmService := service.NewLLMService(&cfg.LLM, zapLog)
	audioService := service.NewAudioService(cfg, zapLog)
	paymentService := service.NewPaymentService(zapLog)
	rechargeService := service.NewRechargeService(rechargeOrderRepo, rechargeRepo, userRepo, paymentService, zapLog)
	orderService := service.NewOrderService(cfg, orderRepo, resultRepo, userService, s3Service, asrService, zapLog)
	resultService := service.NewTranscriptionResultService(resultRepo, zapLog)
	subtitleService := service.NewSubtitleService(cfg, s3Service, resultRepo, zapLog)
	meetingNoteService := service.NewMeetingNoteService(cfg, llmService, s3Service, summaryRepo, resultRepo, zapLog)

	// 初始化 Middleware
	authMiddleware := middleware.NewAuthMiddleware(userService, &cfg.Auth)
	limiter := middleware.NewQPSLimiter(10, 20, userService, &cfg.Pricing)
	sidMiddleware := middleware.NewSidMiddleware(cfg.Auth.SidSecret)

	// 初始化 Handler
	userHandler := handler.NewUserHandler(userService, authMiddleware)
	orderHandler := handler.NewOrderHandler(orderService, rechargeService, resultService, audioService, s3Service, asrService, subtitleService, meetingNoteService)
	paymentCallbackHandler := handler.NewPaymentCallbackHandler(rechargeService)
	rechargeHandler := handler.NewRechargeHandler(rechargeService)

	// 启动 ASR 轮询 worker
	asrWorker := service.NewASRWorker(orderService, asrService, s3Service, cfg, zapLog)
	go asrWorker.Start()

	// 启动服务器
	router := setupRouter(zapLog, userHandler, orderHandler, paymentCallbackHandler, rechargeHandler, authMiddleware, limiter, sidMiddleware)

	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		zapLog.Info("server started", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLog.Fatal("server failed", zap.Error(err))
		}
	}()

	// 等待信号
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	zapLog.Info("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zapLog.Fatal("server shutdown failed", zap.Error(err))
	}

	zapLog.Info("server stopped")
}

func setupRouter(
	zapLog *zap.Logger,
	userHandler *handler.UserHandler,
	orderHandler *handler.OrderHandler,
	paymentCallbackHandler *handler.PaymentCallbackHandler,
	rechargeHandler *handler.RechargeHandler,
	authMiddleware *middleware.AuthMiddleware,
	limiter *middleware.QPSLimiter,
	sidMiddleware *middleware.SidMiddleware,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Sid"},
		ExposeHeaders:    []string{"Content-Length", "X-Sid"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.Use(sidMiddleware.SidMiddleware())
	router.Use(middleware.LoggerMiddleware(zapLog))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/register", userHandler.Register)
	router.POST("/login", userHandler.Login)

	auth := router.Group("")
	auth.Use(authMiddleware.AuthMiddleware())
	auth.Use(middleware.QPSLimitMiddleware(limiter))
	{
		auth.POST("/logout", userHandler.Logout)
		auth.GET("/tokens", userHandler.ListTokens)
		auth.DELETE("/tokens/:token_id", userHandler.DeleteToken)
		auth.POST("/recharge", rechargeHandler.CreateRecharge)
		auth.GET("/recharge/:recharge_no", rechargeHandler.GetRecharge)
		auth.GET("/recharges", rechargeHandler.ListRechargeOrders)
		auth.POST("/orders", orderHandler.CreateOrder)
		auth.POST("/orders/stream", orderHandler.CreateOrderStream)
		auth.GET("/orders/:order_no", orderHandler.GetOrder)
		auth.GET("/orders", orderHandler.ListOrders)
		auth.DELETE("/orders/:order_no", orderHandler.CancelOrder)
		auth.GET("/results/:order_no", orderHandler.GetResult)
		auth.POST("/subtitles/:order_no", orderHandler.GenerateSubtitle)
		auth.GET("/subtitles/:order_no", orderHandler.GetSubtitle)
		auth.POST("/meeting_notes/:order_no", orderHandler.GenerateMeetingNote)
		auth.GET("/meeting_notes/:order_no", orderHandler.GetMeetingNote)
		auth.GET("/orders/insufficient", orderHandler.ListInsufficientOrders)
		auth.POST("/orders/retry", orderHandler.RetryInsufficientOrder)
		auth.GET("/orders/:order_no/cost", orderHandler.GetOrderCost)
		auth.GET("/bills", orderHandler.GetBills)
	}

	router.POST("/callback/alipay", paymentCallbackHandler.AlipayCallback)
	router.POST("/callback/wechat", paymentCallbackHandler.WechatCallback)

	return router
}
