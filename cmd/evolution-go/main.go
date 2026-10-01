package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomessguii/logger"
	"github.com/joho/godotenv"
	"go.mau.fi/whatsmeow"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	call_handler "github.com/evolution-foundation/evolution-go/pkg/call/handler"
	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	call_stream "github.com/evolution-foundation/evolution-go/pkg/call/stream"
	chat_handler "github.com/evolution-foundation/evolution-go/pkg/chat/handler"
	chat_service "github.com/evolution-foundation/evolution-go/pkg/chat/service"
	community_handler "github.com/evolution-foundation/evolution-go/pkg/community/handler"
	community_service "github.com/evolution-foundation/evolution-go/pkg/community/service"
	config "github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/evolution-foundation/evolution-go/pkg/core"
	producer_interfaces "github.com/evolution-foundation/evolution-go/pkg/events/interfaces"
	nats_producer "github.com/evolution-foundation/evolution-go/pkg/events/nats"
	rabbitmq_producer "github.com/evolution-foundation/evolution-go/pkg/events/rabbitmq"
	webhook_producer "github.com/evolution-foundation/evolution-go/pkg/events/webhook"
	websocket_producer "github.com/evolution-foundation/evolution-go/pkg/events/websocket"
	group_handler "github.com/evolution-foundation/evolution-go/pkg/group/handler"
	group_service "github.com/evolution-foundation/evolution-go/pkg/group/service"
	instance_handler "github.com/evolution-foundation/evolution-go/pkg/instance/handler"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	label_handler "github.com/evolution-foundation/evolution-go/pkg/label/handler"
	label_model "github.com/evolution-foundation/evolution-go/pkg/label/model"
	label_repository "github.com/evolution-foundation/evolution-go/pkg/label/repository"
	label_service "github.com/evolution-foundation/evolution-go/pkg/label/service"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	message_handler "github.com/evolution-foundation/evolution-go/pkg/message/handler"
	message_model "github.com/evolution-foundation/evolution-go/pkg/message/model"
	message_repository "github.com/evolution-foundation/evolution-go/pkg/message/repository"
	message_service "github.com/evolution-foundation/evolution-go/pkg/message/service"
	auth_middleware "github.com/evolution-foundation/evolution-go/pkg/middleware"
	newsletter_handler "github.com/evolution-foundation/evolution-go/pkg/newsletter/handler"
	newsletter_service "github.com/evolution-foundation/evolution-go/pkg/newsletter/service"
	passkey_handler "github.com/evolution-foundation/evolution-go/pkg/passkey/handler"
	poll_handler "github.com/evolution-foundation/evolution-go/pkg/poll/handler"
	routes "github.com/evolution-foundation/evolution-go/pkg/routes"
	"github.com/evolution-foundation/evolution-go/pkg/safemap"
	send_handler "github.com/evolution-foundation/evolution-go/pkg/sendMessage/handler"
	send_service "github.com/evolution-foundation/evolution-go/pkg/sendMessage/service"
	server_handler "github.com/evolution-foundation/evolution-go/pkg/server/handler"
	storage_interfaces "github.com/evolution-foundation/evolution-go/pkg/storage/interfaces"
	minio_storage "github.com/evolution-foundation/evolution-go/pkg/storage/minio"
	user_handler "github.com/evolution-foundation/evolution-go/pkg/user/handler"
	user_service "github.com/evolution-foundation/evolution-go/pkg/user/service"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	amqp "github.com/rabbitmq/amqp091-go"
)

var devMode = flag.Bool("dev", false, "Enable development mode")

var version = "0.0.0"

func init() {
	// ldflags -X main.version= sets this at compile time.
	// If not set (or still default), try reading from VERSION file.
	if version == "0.0.0" {
		if v, err := os.ReadFile("VERSION"); err == nil {
			if trimmed := strings.TrimSpace(string(v)); trimmed != "" {
				version = trimmed
			}
		}
	}
}

func setupRouter(db *gorm.DB, authDB *sql.DB, sqliteDB *sql.DB, config *config.Config, conn *amqp.Connection, exPath string, runtimeCtx *core.RuntimeContext) *gin.Engine {
	killChannel := safemap.New[chan bool]()
	clientPointer := safemap.New[*whatsmeow.Client]()

	loggerWrapper := logger_wrapper.NewLoggerManager(config)

	var rabbitmqProducer producer_interfaces.Producer
	if conn != nil {
		logger.LogInfo("RabbitMQ enabled")
		rabbitmqProducer = rabbitmq_producer.NewRabbitMQProducer(
			conn,
			config.AmqpGlobalEnabled,
			config.AmqpGlobalEvents,
			config.AmqpSpecificEvents,
			config.AmqpUrl,
			loggerWrapper,
		)
	} else {
		// Even if initial connection failed, pass the URL so reconnection can work
		rabbitmqProducer = rabbitmq_producer.NewRabbitMQProducer(
			nil,
			config.AmqpGlobalEnabled,
			config.AmqpGlobalEvents,
			config.AmqpSpecificEvents,
			config.AmqpUrl, // Keep the URL for reconnection attempts
			loggerWrapper,
		)
	}

	var natsProducer producer_interfaces.Producer
	if config.NatsUrl != "" {
		logger.LogInfo("NATS enabled")
		natsProducer = nats_producer.NewNatsProducer(
			config.NatsUrl,
			config.NatsGlobalEnabled,
			config.NatsGlobalEvents,
			loggerWrapper,
		)
	} else {
		natsProducer = nats_producer.NewNatsProducer(
			"",
			false,
			nil,
			loggerWrapper,
		)
	}

	webhookProducer := webhook_producer.NewWebhookProducer(config.WebhookUrl, loggerWrapper)
	websocketProducer := websocket_producer.NewWebsocketProducer(loggerWrapper)

	// Cria filas globais se o RabbitMQ global estiver habilitado
	if config.AmqpGlobalEnabled && conn != nil {
		logger.LogInfo("Creating global RabbitMQ queues...")
		if err := rabbitmqProducer.CreateGlobalQueues(); err != nil {
			logger.LogError("Failed to create global RabbitMQ queues: %v", err)
		} else {
			logger.LogInfo("Global RabbitMQ queues created successfully")
		}
	}

	var mediaStorage storage_interfaces.MediaStorage
	var err error
	if config.MinioEnabled {
		mediaStorage, err = minio_storage.NewMinioMediaStorage(
			config.MinioEndpoint,
			config.MinioAccessKey,
			config.MinioSecretKey,
			config.MinioBucket,
			config.MinioRegion,
			config.MinioUseSSL,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	instanceRepository := instance_repository.NewInstanceRepository(db)
	messageRepository := message_repository.NewMessageRepository(db)
	labelRepository := label_repository.NewLabelRepository(db)

	whatsmeowService := whatsmeow_service.NewWhatsmeowService(
		instanceRepository,
		authDB,
		message_repository.NewMessageRepository(db),
		labelRepository,
		config,
		killChannel,
		clientPointer,
		rabbitmqProducer,
		webhookProducer,
		websocketProducer,
		sqliteDB,
		exPath,
		mediaStorage,
		natsProducer,
		loggerWrapper,
	)
	instanceService := instance_service.NewInstanceService(
		instanceRepository,
		killChannel,
		clientPointer,
		whatsmeowService,
		config,
		loggerWrapper,
	)
	sendMessageService := send_service.NewSendService(clientPointer, whatsmeowService, config, loggerWrapper)
	userService := user_service.NewUserService(clientPointer, whatsmeowService, loggerWrapper)
	messageService := message_service.NewMessageService(clientPointer, messageRepository, whatsmeowService, loggerWrapper)
	chatService := chat_service.NewChatService(clientPointer, whatsmeowService, loggerWrapper)
	groupService := group_service.NewGroupService(clientPointer, whatsmeowService, loggerWrapper)
	callTickets := call_stream.NewTickets()
	callService := call_service.NewCallService(clientPointer, whatsmeowService, callTickets, loggerWrapper)
	communityService := community_service.NewCommunityService(clientPointer, whatsmeowService, loggerWrapper)
	labelService := label_service.NewLabelService(clientPointer, whatsmeowService, labelRepository, loggerWrapper)
	newsletterService := newsletter_service.NewNewsletterService(clientPointer, whatsmeowService, loggerWrapper)

	// NOVO: PollHandler usando PollService já inicializado no whatsmeowService (evita dupla inicialização)
	pollHandler := poll_handler.NewPollHandler(whatsmeowService.GetPollService(), loggerWrapper)

	// gin.Default() prints the query string as it came, which put credentials such as
	// /ws?token=<GLOBAL_API_KEY> in the access log; AccessLog redacts them.
	r := gin.New()
	r.Use(auth_middleware.AccessLog(), gin.Recovery())

	// Files above this size are kept on disk while a multipart body is parsed, not in memory.
	r.MaxMultipartMemory = 8 << 20

	// CORS middleware — must be before everything else (the only CORS handler: routes.go
	// used to add the same headers a second time)
	r.Use(auth_middleware.CORS(config.CorsOrigins))

	// No request may make the process buffer an unbounded body.
	r.Use(auth_middleware.LimitBody(config.MaxBodyBytes, config.MaxMediaBodyBytes))

	r.Use(core.GateMiddleware(runtimeCtx))

	// License routes (always accessible, even without license)
	core.LicenseRoutes(r, runtimeCtx)

	// Passkey ceremony routes — PUBLIC (called by the browser extension from the
	// web.whatsapp.com origin, gated only by an opaque ephemeral token).
	passkey_handler.RegisterRoutes(r, whatsmeowService)

	// Audio stream of calls — authorised by a one-time ticket instead of the apikey
	// header, because a browser cannot set that header on a WebSocket.
	call_stream.RegisterRoutes(r, whatsmeowService.CallEngine(), callTickets, call_stream.Config{AllowedOrigins: config.CallStreamOrigins})

	routes.NewRouter(
		auth_middleware.NewMiddleware(config, instanceService),
		instance_handler.NewInstanceHandler(instanceService, config),
		user_handler.NewUserHandler(userService),
		send_handler.NewSendHandler(sendMessageService),
		message_handler.NewMessageHandler(messageService),
		chat_handler.NewChatHandler(chatService),
		group_handler.NewGroupHandler(groupService),
		call_handler.NewCallHandler(callService),
		community_handler.NewCommunityHandler(communityService),
		label_handler.NewLabelHandler(labelService),
		newsletter_handler.NewNewsletterHandler(newsletterService),
		pollHandler,
		server_handler.NewServerHandler(
			server_handler.HealthCheck{Name: "usersDb", DB: usersSQLDB(db)},
			server_handler.HealthCheck{Name: "authDb", DB: firstSQLDB(authDB, sqliteDB)},
		),
	).AssignRoutes(r)

	if config.ConnectOnStartup {
		go whatsmeowService.ConnectOnStartup(config.ClientName)
	}

	// Optional Go profiler (goroutine/heap/cpu), for hunting leaks. Off by default and
	// only reachable with the GLOBAL API key.
	if config.PprofEnabled {
		logger.LogWarn("ENABLE_PPROF is set: /debug/pprof is exposed behind the global API key")
		pp := r.Group("/debug/pprof", auth_middleware.NewMiddleware(config, instanceService).AuthAdmin)
		pp.GET("/", gin.WrapF(pprof.Index))
		pp.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pp.GET("/profile", gin.WrapF(pprof.Profile))
		pp.GET("/symbol", gin.WrapF(pprof.Symbol))
		pp.GET("/trace", gin.WrapF(pprof.Trace))
		for _, name := range []string{"goroutine", "heap", "allocs", "block", "mutex", "threadcreate"} {
			pp.GET("/"+name, gin.WrapH(pprof.Handler(name)))
		}
	}

	r.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		instanceId := c.Query("instanceId")

		// Constant-time compare, and never log the token that was sent: it may be a
		// near-miss of the real global key.
		if subtle.ConstantTimeCompare([]byte(token), []byte(config.GlobalApiKey)) != 1 {
			logger.LogError("Token inválido na conexão WebSocket")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			return
		}

		websocket_producer.ServeWs(c.Writer, c.Request, instanceId, websocketProducer)
	})

	return r
}

// usersSQLDB returns the *sql.DB behind the users gorm handle (nil if unavailable).
func usersSQLDB(db *gorm.DB) *sql.DB {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil
	}
	return sqlDB
}

// firstSQLDB returns the first non-nil database.
func firstSQLDB(dbs ...*sql.DB) *sql.DB {
	for _, d := range dbs {
		if d != nil {
			return d
		}
	}
	return nil
}

func migrate(db *gorm.DB) {
	err := db.AutoMigrate(&instance_model.Instance{}, &message_model.Message{}, &label_model.Label{})

	if err != nil {
		log.Fatal(err)
	}

	// message_id used to be unique on its own, which made two instances that receive the
	// same message (same group) overwrite each other's row. The key is
	// (instance_id, message_id) now; drop the old constraint.
	if db.Migrator().HasConstraint(&message_model.Message{}, "uni_messages_message_id") {
		if err := db.Migrator().DropConstraint(&message_model.Message{}, "uni_messages_message_id"); err != nil {
			log.Fatal(err)
		}
	}
}

func initAuthDB(config *config.Config) (*sql.DB, string, error) {
	if config.PostgresAuthDB != "" {
		return nil, "", nil
	}

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	dbDirectory := exPath + "/dbdata"
	_, err = os.Stat(dbDirectory)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dbDirectory, 0751)
		if errDir != nil {
			panic("Could not create dbdata directory")
		}
	}

	db, err := sql.Open("sqlite", exPath+"/dbdata/users.db?_pragma=foreign_keys(1)&_busy_timeout=3000")
	if err != nil {
		return nil, "", err
	}

	return db, exPath, nil
}

func initPostgresAuthDB(config *config.Config) (*sql.DB, error) {
	if config.PostgresAuthDB == "" {
		return nil, nil
	}

	if err := config.EnsureDBExists(config.PostgresAuthDB); err != nil {
		logger.LogWarn("Auto-setup auth DB failed (will try connecting anyway): %v", err)
	}

	db, err := sql.Open("postgres", config.PostgresAuthDB)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco AUTH PostgreSQL: %v", err)
	}

	// Configurar pool de conexões para evitar conexões ociosas não fechadas
	db.SetMaxOpenConns(25)                 // Máximo de 25 conexões abertas simultaneamente
	db.SetMaxIdleConns(5)                  // Máximo de 5 conexões ociosas no pool
	db.SetConnMaxLifetime(5 * time.Minute) // Reconectar após 5 minutos para evitar timeouts
	db.SetConnMaxIdleTime(1 * time.Minute) // Fechar conexões ociosas após 1 minuto

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("erro ao pingar banco AUTH PostgreSQL: %v", err)
	}

	logger.LogInfo("Conectado ao banco AUTH PostgreSQL com pool configurado")
	return db, nil
}

// @title Evolution GO
// @version 1.0
// @description Evolution GO - whatsmeow
func main() {
	flag.Parse()
	if *devMode {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal(err)
		}
	}

	// Release mode unless the operator chose otherwise (GIN_MODE): debug mode prints every
	// route at boot and adds work per request.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfg := config.Load()
	if cfg.WebhookIncludeToken {
		logger.LogWarn("[CONFIG] Events carry the instance token (\"instanceToken\"): every webhook, queue and websocket consumer can use it as the instance API key. Leave WEBHOOK_INCLUDE_TOKEN unset (false) unless an integration needs it")
	}

	logger.LogInfo("Starting Evolution GO version %s", version)

	startTime := time.Now()

	db, err := cfg.CreateUsersDB()
	if err != nil {
		log.Fatal(err)
	}

	// Inicializar PostgreSQL AUTH
	authDB, err := initPostgresAuthDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if authDB != nil {
		defer authDB.Close()
	}

	// Manter inicialização do SQLite
	sqliteDB, exPath, err := initAuthDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if sqliteDB != nil {
		defer sqliteDB.Close()
	}

	migrate(db)

	// Initialize core DB + license runtime
	core.SetDB(db)
	if err := core.MigrateDB(); err != nil {
		log.Fatal("Failed to migrate runtime_configs: ", err)
	}
	tier := "evolution-go"
	runtimeCtx := core.InitializeRuntime(tier, version, cfg.GlobalApiKey)

	var conn *amqp.Connection

	if cfg.AmqpUrl != "" {
		logger.LogInfo("Attempting to connect to RabbitMQ...")

		// Create connection with heartbeat to prevent timeouts
		amqpConfig := amqp.Config{
			Heartbeat: 30 * time.Second, // Send heartbeat every 30 seconds
			Locale:    "en_US",
		}

		conn, err = amqp.DialConfig(cfg.AmqpUrl, amqpConfig)
		if err != nil {
			logger.LogError("Failed to connect to RabbitMQ, err: %v", err)
			logger.LogInfo("RabbitMQ producer will be created with reconnection capability")
		} else {
			logger.LogInfo("Successfully connected to RabbitMQ with heartbeat enabled")
			defer func(conn *amqp.Connection) {
				err := conn.Close()
				if err != nil {
					logger.LogError("Failed to close RabbitMQ connection, err: %v", err)
				}
			}(conn)
		}
	} else {
		logger.LogInfo("RabbitMQ URL not configured, skipping RabbitMQ connection")
	}

	r := setupRouter(db, authDB, sqliteDB, cfg, conn, exPath, runtimeCtx)

	// Graceful shutdown with heartbeat
	heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
	defer heartbeatCancel()

	core.StartHeartbeat(heartbeatCtx, runtimeCtx, startTime)

	// Without these a client could hold a connection forever by sending its headers or
	// body one byte at a time. There is no WriteTimeout on purpose: sends with a typing
	// delay, QR polling and the websockets answer for a long time.
	srv := &http.Server{
		Addr:              ":" + os.Getenv("SERVER_PORT"),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Minute, // a 100 MB upload over a slow link
		IdleTimeout:       2 * time.Minute,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.LogInfo("Iniciando servidor na porta %s", os.Getenv("SERVER_PORT"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	logger.LogInfo("[SHUTDOWN] Signal received, shutting down...")

	// Stop heartbeat loop
	heartbeatCancel()

	core.Shutdown(runtimeCtx)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.LogError("[SHUTDOWN] Server forced to shutdown: %v", err)
	}

	logger.LogInfo("[SHUTDOWN] Server exited")
}
