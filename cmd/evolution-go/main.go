package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomessguii/logger"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	call_handler "github.com/EvolutionAPI/evolution-go/pkg/call/handler"
	call_service "github.com/EvolutionAPI/evolution-go/pkg/call/service"
	chat_handler "github.com/EvolutionAPI/evolution-go/pkg/chat/handler"
	chat_service "github.com/EvolutionAPI/evolution-go/pkg/chat/service"
	community_handler "github.com/EvolutionAPI/evolution-go/pkg/community/handler"
	community_service "github.com/EvolutionAPI/evolution-go/pkg/community/service"
	config "github.com/EvolutionAPI/evolution-go/pkg/config"
	"github.com/EvolutionAPI/evolution-go/pkg/core"
	producer_interfaces "github.com/EvolutionAPI/evolution-go/pkg/events/interfaces"
	nats_producer "github.com/EvolutionAPI/evolution-go/pkg/events/nats"
	rabbitmq_producer "github.com/EvolutionAPI/evolution-go/pkg/events/rabbitmq"
	webhook_producer "github.com/EvolutionAPI/evolution-go/pkg/events/webhook"
	websocket_producer "github.com/EvolutionAPI/evolution-go/pkg/events/websocket"
	group_handler "github.com/EvolutionAPI/evolution-go/pkg/group/handler"
	group_service "github.com/EvolutionAPI/evolution-go/pkg/group/service"
	instance_handler "github.com/EvolutionAPI/evolution-go/pkg/instance/handler"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	instance_service "github.com/EvolutionAPI/evolution-go/pkg/instance/service"
	label_handler "github.com/EvolutionAPI/evolution-go/pkg/label/handler"
	label_model "github.com/EvolutionAPI/evolution-go/pkg/label/model"
	label_repository "github.com/EvolutionAPI/evolution-go/pkg/label/repository"
	label_service "github.com/EvolutionAPI/evolution-go/pkg/label/service"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"github.com/EvolutionAPI/evolution-go/pkg/mcp"
	message_handler "github.com/EvolutionAPI/evolution-go/pkg/message/handler"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	message_repository "github.com/EvolutionAPI/evolution-go/pkg/message/repository"
	message_service "github.com/EvolutionAPI/evolution-go/pkg/message/service"
	auth_middleware "github.com/EvolutionAPI/evolution-go/pkg/middleware"
	newsletter_handler "github.com/EvolutionAPI/evolution-go/pkg/newsletter/handler"
	newsletter_service "github.com/EvolutionAPI/evolution-go/pkg/newsletter/service"
	passkey_handler "github.com/EvolutionAPI/evolution-go/pkg/passkey/handler"
	poll_handler "github.com/EvolutionAPI/evolution-go/pkg/poll/handler"
	routes "github.com/EvolutionAPI/evolution-go/pkg/routes"
	send_handler "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/handler"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
	server_handler "github.com/EvolutionAPI/evolution-go/pkg/server/handler"
	storage_interfaces "github.com/EvolutionAPI/evolution-go/pkg/storage/interfaces"
	minio_storage "github.com/EvolutionAPI/evolution-go/pkg/storage/minio"
	user_handler "github.com/EvolutionAPI/evolution-go/pkg/user/handler"
	user_service "github.com/EvolutionAPI/evolution-go/pkg/user/service"
	voip_transport "github.com/EvolutionAPI/evolution-go/pkg/voip/transport"
	whatsmeow_registry "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/registry"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
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
	// One registry for every live WhatsApp client, shared by all the services
	// below. It replaces the bare maps that used to be passed around and
	// written without synchronisation.
	clients := whatsmeow_registry.New()

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
		clients,
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
		clients,
		whatsmeowService,
		config,
		loggerWrapper,
	)
	sendMessageService := send_service.NewSendService(clients, whatsmeowService, config, loggerWrapper)
	userService := user_service.NewUserService(clients, whatsmeowService, loggerWrapper)
	messageService := message_service.NewMessageService(clients, messageRepository, whatsmeowService, loggerWrapper)
	chatService := chat_service.NewChatService(clients, whatsmeowService, loggerWrapper)
	groupService := group_service.NewGroupService(clients, whatsmeowService, loggerWrapper)
	callService := call_service.NewCallService(clients, whatsmeowService, loggerWrapper)
	communityService := community_service.NewCommunityService(clients, whatsmeowService, loggerWrapper)
	labelService := label_service.NewLabelService(clients, whatsmeowService, labelRepository, loggerWrapper)
	newsletterService := newsletter_service.NewNewsletterService(clients, whatsmeowService, loggerWrapper)

	// NOVO: PollHandler usando PollService já inicializado no whatsmeowService (evita dupla inicialização)
	pollHandler := poll_handler.NewPollHandler(whatsmeowService.GetPollService(), loggerWrapper)

	r := gin.Default()

	// CORS middleware — must be before everything else
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With, apikey, ApiKey")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}
		c.Next()
	})

	r.Use(core.GateMiddleware(runtimeCtx))

	// License routes (always accessible, even without license)
	core.LicenseRoutes(r, runtimeCtx)

	// Passkey ceremony routes — PUBLIC (called by the browser extension from the
	// web.whatsapp.com origin, gated only by an opaque ephemeral ceremony token).
	passkey_handler.RegisterRoutes(r, whatsmeowService)

	// Route call media through each instance's proxy. Only SOCKS5 can carry the
	// UDP media path; with an HTTP proxy the call still rings (signaling rides
	// the whatsmeow websocket, which already uses the proxy) but audio cannot.
	whatsmeowService.CallRegistry().SetProxyResolver(func(instanceID string) *voip_transport.ProxyConfig {
		proxyConfig, err := instanceService.GetProxy(instanceID)
		if err != nil || proxyConfig == nil {
			return nil
		}
		return &voip_transport.ProxyConfig{
			Protocol: proxyConfig.Protocol,
			Host:     proxyConfig.Host,
			Port:     proxyConfig.Port,
			Username: proxyConfig.Username,
			Password: proxyConfig.Password,
		}
	})

	// Model Context Protocol endpoint — lets Claude, ChatGPT and other MCP
	// clients send WhatsApp messages and search the message archive.
	mcp.RegisterRoutes(r, mcp.NewServer(
		instanceService,
		sendMessageService,
		messageService,
		callService,
		whatsmeowService,
		config,
		loggerWrapper,
	))

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
		server_handler.NewServerHandler(),
	).AssignRoutes(r)

	if config.ConnectOnStartup {
		go whatsmeowService.ConnectOnStartup(config.ClientName)
	}

	r.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		instanceId := c.Query("instanceId")

		if token != config.GlobalApiKey {
			logger.LogError("Token inválido: %s", token)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			return
		}

		websocket_producer.ServeWs(c.Writer, c.Request, instanceId, websocketProducer)
	})

	return r
}

func migrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&instance_model.Instance{},
		&message_model.Message{},
		&message_model.ArchivedMessage{},
		&label_model.Label{},
	)

	if err != nil {
		log.Fatal(err)
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

// @title Evolution GO API
// @version 1.0
// @description API HTTP para WhatsApp construida sobre whatsmeow, com suporte a multiplas
// @description instancias, mensagens interativas, chamadas de voz, webhooks e MCP.
// @description
// @description ## Autenticacao
// @description
// @description Toda requisicao autenticada usa o header `apikey`. O valor depende do escopo:
// @description
// @description - **Chave global** (`GLOBAL_API_KEY`): administra instancias — criar, listar,
// @description   apagar, configurar proxy, ler logs. Sao as rotas sob `/instance` marcadas
// @description   como admin.
// @description - **Token da instancia**: opera uma instancia — enviar mensagens, ler chats,
// @description   grupos, chamadas. O token e devolvido na criacao da instancia.
// @description
// @description Use o botao **Authorize** para informar a chave; ela sera enviada no header
// @description `apikey` em todas as chamadas de teste.
// @description
// @description ## Webhooks
// @description
// @description Cada instancia pode ter varias URLs de webhook (`/instance/webhooks/{id}`) e
// @description todas recebem o mesmo payload. A entrega e assincrona, com ate 5 tentativas e
// @description espera exponencial. Respostas 4xx nao sao reenviadas — elas indicam recusa, nao
// @description indisponibilidade; 5xx, 408 e 429 sao reenviadas.
// @description
// @description ## Formato dos erros
// @description
// @description Falhas retornam `{"error": "mensagem"}` com o status HTTP correspondente.
// @BasePath /
// @tag.name Instance
// @tag.description Ciclo de vida da instancia: criar, conectar, QR, proxy, webhooks, logs.
// @tag.name Send Message
// @tag.description Envio de mensagens — texto, midia, botoes, lista, carrossel, enquete, status.
// @tag.name Message
// @tag.description Acoes sobre mensagens ja enviadas: reagir, editar, apagar, marcar como lida.
// @tag.name Chat
// @tag.description Conversas: presenca, arquivar, fixar, silenciar, historico.
// @tag.name Group
// @tag.description Grupos: criar, participantes, permissoes, convites.
// @tag.name Community
// @tag.description Comunidades e seus grupos vinculados.
// @tag.name Newsletter
// @tag.description Canais (newsletters).
// @tag.name Label
// @tag.description Etiquetas do WhatsApp Business.
// @tag.name Call
// @tag.description Chamadas de voz: ligar, atender, desligar e audio bidirecional.
// @tag.name User
// @tag.description Perfil, foto, status e privacidade da conta conectada.
// @tag.name Polls
// @tag.description Enquetes e seus votos.
// @tag.name Passkey
// @tag.description Cerimonia WebAuthn usada quando a conta exige passkey para parear.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name apikey
func main() {
	flag.Parse()
	if *devMode {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal(err)
		}
	}

	cfg := config.Load()

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

	srv := &http.Server{
		Addr:    ":" + os.Getenv("SERVER_PORT"),
		Handler: r,
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

	// The whatsmeow device store is one pool shared by every instance, opened
	// lazily on the first connection. Closing it here releases its Postgres
	// connections instead of leaving them for the server to time out.
	if err := whatsmeow_service.CloseDeviceStore(); err != nil {
		logger.LogError("[SHUTDOWN] Failed to close the device store: %v", err)
	}

	logger.LogInfo("[SHUTDOWN] Server exited")
}
