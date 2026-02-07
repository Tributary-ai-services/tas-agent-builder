package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	Router    RouterConfig    `json:"router"`
	Auth      AuthConfig      `json:"auth"`
	Logging   LoggingConfig   `json:"logging"`
	DeepLake  DeepLakeConfig  `json:"deeplake"`
	AudiModal AudiModalConfig `json:"audimodal"`
	Aether    AetherConfig    `json:"aether"`
	Redis     RedisConfig     `json:"redis"`
	MCP       MCPConfig       `json:"mcp"`
}

// MCPConfig holds configuration for MCP tool integration
type MCPConfig struct {
	ServerURL         string `json:"server_url"`
	Timeout           int    `json:"timeout"`
	MaxToolIterations int    `json:"max_tool_iterations"`
	Enabled           bool   `json:"enabled"`
}

type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
}

type DatabaseConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	SSLMode      string `json:"ssl_mode"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
	MaxLifetime  int    `json:"max_lifetime"`
}

type RouterConfig struct {
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Timeout    int    `json:"timeout"`
	MaxRetries int    `json:"max_retries"`
}

type AuthConfig struct {
	JWTSecret     string   `json:"jwt_secret"`
	JWTExpiration int      `json:"jwt_expiration"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type LoggingConfig struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
}

// DeepLakeConfig holds configuration for DeepLake vector search API
type DeepLakeConfig struct {
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	Timeout           int    `json:"timeout"`
	DefaultDataset    string `json:"default_dataset"`
	UseDefaultDataset bool   `json:"use_default_dataset"`
}

// AudiModalConfig holds configuration for AudiModal document processing API
type AudiModalConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Timeout int    `json:"timeout"`
}

// AetherConfig holds configuration for Aether-BE API (Neo4j integration)
type AetherConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Timeout int    `json:"timeout"`
}

// RedisConfig holds configuration for Redis caching
type RedisConfig struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Password          string `json:"password"`
	DB                int    `json:"db"`
	ContextCacheTTL   int    `json:"context_cache_ttl"`   // TTL for document context cache in seconds
	EnableContextCache bool  `json:"enable_context_cache"`
}

func LoadConfig() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvAsInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvAsInt("SERVER_READ_TIMEOUT", 30),
			WriteTimeout: getEnvAsInt("SERVER_WRITE_TIMEOUT", 30),
			IdleTimeout:  getEnvAsInt("SERVER_IDLE_TIMEOUT", 60),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnvAsInt("DB_PORT", 5432),
			User:         getEnv("DB_USER", "tasuser"),
			Password:     getEnv("DB_PASSWORD", "taspassword"),
			Name:         getEnv("DB_NAME", "tas_shared"),
			SSLMode:      getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			MaxLifetime:  getEnvAsInt("DB_MAX_LIFETIME", 300),
		},
		Router: RouterConfig{
			BaseURL:    getEnv("ROUTER_BASE_URL", "http://localhost:8081"),
			APIKey:     getEnv("ROUTER_API_KEY", ""),
			Timeout:    getEnvAsInt("ROUTER_TIMEOUT", 30),
			MaxRetries: getEnvAsInt("ROUTER_MAX_RETRIES", 3),
		},
		Auth: AuthConfig{
			JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration:  getEnvAsInt("JWT_EXPIRATION", 3600),
			AllowedOrigins: getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			Output:     getEnv("LOG_OUTPUT", "stdout"),
			MaxSize:    getEnvAsInt("LOG_MAX_SIZE", 100),
			MaxBackups: getEnvAsInt("LOG_MAX_BACKUPS", 3),
			MaxAge:     getEnvAsInt("LOG_MAX_AGE", 7),
			Compress:   getEnvAsBool("LOG_COMPRESS", true),
		},
		DeepLake: DeepLakeConfig{
			BaseURL:           getEnv("DEEPLAKE_BASE_URL", "http://localhost:8000"),
			APIKey:            getEnv("DEEPLAKE_API_KEY", ""),
			Timeout:           getEnvAsInt("DEEPLAKE_TIMEOUT", 30),
			DefaultDataset:    getEnv("DEEPLAKE_DEFAULT_DATASET", "documents"),
			UseDefaultDataset: getEnvAsBool("DEEPLAKE_USE_DEFAULT_DATASET", true),
		},
		AudiModal: AudiModalConfig{
			BaseURL: getEnv("AUDIMODAL_BASE_URL", "http://localhost:8084"),
			APIKey:  getEnv("AUDIMODAL_API_KEY", ""),
			Timeout: getEnvAsInt("AUDIMODAL_TIMEOUT", 30),
		},
		Aether: AetherConfig{
			BaseURL: getEnv("AETHER_BASE_URL", "http://localhost:8080"),
			APIKey:  getEnv("AETHER_INTERNAL_API_KEY", ""),
			Timeout: getEnvAsInt("AETHER_TIMEOUT", 30),
		},
		Redis: RedisConfig{
			Host:               getEnv("REDIS_HOST", "localhost"),
			Port:               getEnvAsInt("REDIS_PORT", 6379),
			Password:           getEnv("REDIS_PASSWORD", ""),
			DB:                 getEnvAsInt("REDIS_DB", 0),
			ContextCacheTTL:    getEnvAsInt("REDIS_CONTEXT_CACHE_TTL", 1800), // 30 minutes default
			EnableContextCache: getEnvAsBool("REDIS_ENABLE_CONTEXT_CACHE", true),
		},
		MCP: MCPConfig{
			ServerURL:         getEnv("MCP_SERVER_URL", "http://napkin-mcp.tas-mcp-servers.svc.cluster.local:8087"),
			Timeout:           getEnvAsInt("MCP_TIMEOUT", 120),
			MaxToolIterations: getEnvAsInt("MCP_MAX_TOOL_ITERATIONS", 10),
			Enabled:           getEnvAsBool("MCP_ENABLED", true),
		},
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func validateConfig(config *Config) error {
	if config.Database.Password == "" {
		return fmt.Errorf("database password is required (DB_PASSWORD)")
	}
	
	if config.Router.BaseURL == "" {
		return fmt.Errorf("router base URL is required (ROUTER_BASE_URL)")
	}
	
	// Router API key is optional - router may not require authentication
	// if config.Router.APIKey == "" {
	//	return fmt.Errorf("router API key is required (ROUTER_API_KEY)")
	// }
	
	if config.Auth.JWTSecret == "your-secret-key-change-in-production" {
		return fmt.Errorf("JWT secret must be changed from default value (JWT_SECRET)")
	}
	
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}