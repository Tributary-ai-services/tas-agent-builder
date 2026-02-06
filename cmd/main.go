package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"github.com/tas-agent-builder/auth"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/handlers"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services/impl"
	"github.com/tas-agent-builder/services/memory"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}
	
	// Initialize database connection
	db, err := initDB(cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	
	// Auto-migrate database schema
	if err := db.AutoMigrate(
		&models.Agent{},
		&models.AgentExecution{},
		&models.AgentUsageStats{},
	); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	
	// Initialize services
	agentService := impl.NewAgentService(db)
	routerService := impl.NewRouterService(&cfg.Router)
	executionService := impl.NewExecutionService(db, routerService)

	// Initialize cache service
	cacheService, err := impl.NewCacheService(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Cache service initialization failed, continuing without caching: %v", err)
		cacheService, _ = impl.NewCacheService(nil) // Disabled cache fallback
	}

	// Initialize document context service
	documentContextService := impl.NewDocumentContextService(
		&cfg.DeepLake,
		&cfg.AudiModal,
		&cfg.Aether,
		cacheService,
	)

	// Initialize Redis client for memory service
	var redisClient *redis.Client
	var memoryService *memory.MemoryServiceImpl
	if cfg.Redis.Host != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		// Test Redis connection
		_, err := redisClient.Ping(context.Background()).Result()
		if err != nil {
			log.Printf("Warning: Redis connection failed, memory service will be disabled: %v", err)
			redisClient = nil
		} else {
			log.Println("Redis connection established for memory service")
		}
	}

	// Initialize memory service if Redis is available
	if redisClient != nil {
		memoryService = memory.NewMemoryService(
			redisClient,
			&cfg.DeepLake,
			&cfg.Router,
			nil, // Use default memory config
		)
		log.Println("Memory service initialized with 3-tier memory system")
	} else {
		log.Println("Memory service disabled (no Redis connection)")
	}

	// Initialize handlers
	agentHandlers := handlers.NewAgentHandlers(agentService, routerService, executionService, documentContextService, cacheService, memoryService)
	routerProxy := handlers.NewRouterProxyHandler(cfg.Router.BaseURL)
	
	// Setup router
	router := setupRouter(agentHandlers, routerProxy, cfg)
	
	// Start server
	srv := &http.Server{
		Addr:    cfg.GetServerAddress(),
		Handler: router,
	}
	
	// Graceful shutdown
	go func() {
		log.Printf("Agent Builder server starting on %s", cfg.GetServerAddress())
		log.Printf("Router URL: %s", cfg.Router.BaseURL)
		log.Printf("Environment: %s", os.Getenv("ENVIRONMENT"))
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()
	
	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	
	log.Println("Server exited")
}

func initDB(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	
	return db, nil
}

func setupRouter(agentHandlers *handlers.AgentHandlers, routerProxy *handlers.RouterProxyHandler, cfg *config.Config) *gin.Engine {
	// Set gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	
	router := gin.New()
	
	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	
	// CORS middleware
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3001", "http://localhost:5173"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))
	
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now(),
			"service":   "agent-builder",
		})
	})
	
	// API v1 routes
	v1 := router.Group("/api/v1")
	
	// Add authentication middleware for API routes
	// Support multiple Keycloak realms and deployment scenarios
	jwtValidator := auth.NewJWTValidator(cfg.Auth.JWTSecret, []string{
		// Aether realm (production)
		"https://keycloak.tas.scharber.com/realms/aether",
		"http://tas-keycloak-shared:8080/realms/aether",
		"http://localhost:8081/realms/aether",
		// Master realm (legacy/fallback)
		"http://localhost:8081/realms/master",
		"http://tas-keycloak-shared:8080/realms/master",
	})
	v1.Use(authMiddleware(jwtValidator))
	
	// Internal agent routes (system tools) - must come BEFORE /:id routes
	// These are available to all authenticated users
	internalAgents := v1.Group("/agents/internal")
	{
		internalAgents.GET("", agentHandlers.GetInternalAgents)
		internalAgents.GET("/:id", agentHandlers.GetInternalAgent)
		internalAgents.POST("/:id/execute", agentHandlers.ExecuteInternalAgent)
	}

	// Agent routes - only add routes that are actually implemented
	agents := v1.Group("/agents")
	{
		agents.POST("", agentHandlers.CreateAgent)
		agents.GET("", agentHandlers.ListAgents)
		agents.GET("/:id", agentHandlers.GetAgent)
		agents.PUT("/:id", agentHandlers.UpdateAgent)
		agents.DELETE("/:id", agentHandlers.DeleteAgent)

		agents.POST("/:id/publish", agentHandlers.PublishAgent)
		agents.POST("/:id/unpublish", agentHandlers.UnpublishAgent)
		agents.POST("/:id/duplicate", agentHandlers.DuplicateAgent)
		agents.POST("/:id/execute", agentHandlers.ExecuteAgent)
	}
	
	// Additional routes that exist in handlers
	v1.GET("/agent-reliability-metrics", agentHandlers.GetAgentReliabilityMetrics)
	v1.POST("/validate-agent-config", agentHandlers.ValidateAgentConfig)
	v1.GET("/agent-config-templates", agentHandlers.GetAgentConfigTemplates)
	v1.GET("/stats/user", agentHandlers.GetUserStats)
	
	// Router proxy endpoints
	routerGroup := v1.Group("/router")
	{
		routerGroup.GET("/providers", routerProxy.GetProviders)
		routerGroup.GET("/providers/:provider/models", routerProxy.GetProviderModels)
	}
	
	return router
}

// authMiddleware validates JWT tokens using RSA signature verification
func authMiddleware(validator *auth.JWTValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		
		// Check for Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}
		
		// Validate token
		claims, err := validator.ValidateToken(authHeader)
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}
		
		// Extract user context from claims
		userID, tenantID := validator.ExtractUserContext(claims)
		
		// Set user context in Gin context
		c.Set("user_id", userID)
		c.Set("tenant_id", tenantID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)
		c.Set("username", claims.PreferredUsername)
		
		log.Printf("Authenticated user: %s (%s)", claims.PreferredUsername, userID)
		
		c.Next()
	}
}