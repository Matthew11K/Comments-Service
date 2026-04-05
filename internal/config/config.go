package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment   string
	HTTP          HTTPConfig
	Storage       StorageConfig
	GraphQL       GraphQLConfig
	Logging       LoggingConfig
	Subscriptions SubscriptionConfig
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type StorageConfig struct {
	Backend  string
	Postgres PostgresConfig
}

type PostgresConfig struct {
	DSN      string
	MaxConns int32
	MinConns int32
}

type GraphQLConfig struct {
	Path                string
	PlaygroundPath      string
	EnablePlayground    bool
	EnableIntrospection bool
	MaxPageSize         int
	MaxDepth            int
	ComplexityLimit     int
	QueryCacheSize      int
	APQCacheSize        int
	ParserTokenLimit    int
	AllowedOrigins      []string
	WebsocketKeepAlive  time.Duration
}

type LoggingConfig struct {
	Level  string
	Format string
}

type SubscriptionConfig struct {
	BufferSize  int
	ChannelName string
}

const environmentProd = "prod"

type Error struct {
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return e.Field + ": " + e.Message
}

func Load(args []string) (Config, error) {
	defaultEnvironment := envString("COMMENT_SERVICE_ENV", "local")
	defaultBackend := envString("COMMENT_SERVICE_STORAGE_BACKEND", "memory")
	defaultAddr := envString("COMMENT_SERVICE_HTTP_ADDR", ":8080")

	flags := flag.NewFlagSet("comment-service", flag.ContinueOnError)
	environment := flags.String("env", defaultEnvironment, "execution profile: local|dev|prod")
	backend := flags.String("storage-backend", defaultBackend, "storage backend: memory|postgres")
	addr := flags.String("http-addr", defaultAddr, "HTTP listen address")

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}

	config := Config{
		Environment: *environment,
		HTTP: HTTPConfig{
			Addr:            *addr,
			ReadTimeout:     envDuration("COMMENT_SERVICE_HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    envDuration("COMMENT_SERVICE_HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     envDuration("COMMENT_SERVICE_HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("COMMENT_SERVICE_HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Storage: StorageConfig{
			Backend: *backend,
			Postgres: PostgresConfig{
				DSN:      envString("COMMENT_SERVICE_POSTGRES_DSN", ""),
				MaxConns: envInt32("COMMENT_SERVICE_POSTGRES_MAX_CONNS", 10),
				MinConns: envInt32("COMMENT_SERVICE_POSTGRES_MIN_CONNS", 1),
			},
		},
		GraphQL: GraphQLConfig{
			Path:                envString("COMMENT_SERVICE_GRAPHQL_PATH", "/query"),
			PlaygroundPath:      envString("COMMENT_SERVICE_GRAPHQL_PLAYGROUND_PATH", "/playground"),
			EnablePlayground:    envBool("COMMENT_SERVICE_GRAPHQL_ENABLE_PLAYGROUND", defaultEnvironment != environmentProd),
			EnableIntrospection: envBool("COMMENT_SERVICE_GRAPHQL_ENABLE_INTROSPECTION", defaultEnvironment != environmentProd),
			MaxPageSize:         envInt("COMMENT_SERVICE_GRAPHQL_MAX_PAGE_SIZE", 100),
			MaxDepth:            envInt("COMMENT_SERVICE_GRAPHQL_MAX_DEPTH", 12),
			ComplexityLimit:     envInt("COMMENT_SERVICE_GRAPHQL_COMPLEXITY_LIMIT", 500),
			QueryCacheSize:      envInt("COMMENT_SERVICE_GRAPHQL_QUERY_CACHE_SIZE", 1000),
			APQCacheSize:        envInt("COMMENT_SERVICE_GRAPHQL_APQ_CACHE_SIZE", 100),
			ParserTokenLimit:    envInt("COMMENT_SERVICE_GRAPHQL_PARSER_TOKEN_LIMIT", 10000),
			AllowedOrigins:      envCSV("COMMENT_SERVICE_GRAPHQL_ALLOWED_ORIGINS", defaultOrigins(defaultEnvironment)),
			WebsocketKeepAlive:  envDuration("COMMENT_SERVICE_GRAPHQL_WEBSOCKET_KEEPALIVE", 15*time.Second),
		},
		Logging: LoggingConfig{
			Level:  envString("COMMENT_SERVICE_LOG_LEVEL", defaultLogLevel(defaultEnvironment)),
			Format: envString("COMMENT_SERVICE_LOG_FORMAT", defaultLogFormat(defaultEnvironment)),
		},
		Subscriptions: SubscriptionConfig{
			BufferSize:  envInt("COMMENT_SERVICE_SUBSCRIPTIONS_BUFFER_SIZE", 16),
			ChannelName: envString("COMMENT_SERVICE_SUBSCRIPTIONS_CHANNEL_NAME", "comment_events"),
		},
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c Config) Validate() error {
	switch c.Environment {
	case "local", "dev", environmentProd:
	default:
		return &Error{Field: "env", Message: "must be one of local, dev, prod"}
	}

	switch c.Storage.Backend {
	case "memory", "postgres":
	default:
		return &Error{Field: "storage-backend", Message: "must be memory or postgres"}
	}

	if c.HTTP.Addr == "" {
		return &Error{Field: "http.addr", Message: "must not be empty"}
	}

	if c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 || c.HTTP.IdleTimeout <= 0 || c.HTTP.ShutdownTimeout <= 0 {
		return &Error{Field: "http", Message: "timeouts must be greater than zero"}
	}

	if c.Storage.Backend == "postgres" && c.Storage.Postgres.DSN == "" {
		return &Error{Field: "postgres.dsn", Message: "must not be empty when postgres backend is selected"}
	}

	if c.Storage.Postgres.MaxConns <= 0 || c.Storage.Postgres.MinConns < 0 || c.Storage.Postgres.MinConns > c.Storage.Postgres.MaxConns {
		return &Error{Field: "postgres.pool", Message: "invalid connection pool settings"}
	}

	if c.GraphQL.Path == "" || c.GraphQL.PlaygroundPath == "" {
		return &Error{Field: "graphql.path", Message: "paths must not be empty"}
	}

	if c.GraphQL.MaxPageSize <= 0 || c.GraphQL.MaxDepth <= 0 || c.GraphQL.ComplexityLimit <= 0 {
		return &Error{Field: "graphql.limits", Message: "limits must be greater than zero"}
	}

	if c.GraphQL.WebsocketKeepAlive <= 0 {
		return &Error{Field: "graphql.websocketKeepAlive", Message: "must be greater than zero"}
	}

	if c.Environment == environmentProd && len(c.GraphQL.AllowedOrigins) == 0 {
		return &Error{Field: "graphql.allowedOrigins", Message: "must not be empty in prod"}
	}

	if c.Subscriptions.BufferSize <= 0 {
		return &Error{Field: "subscriptions.bufferSize", Message: "must be greater than zero"}
	}

	if c.Subscriptions.ChannelName == "" {
		return &Error{Field: "subscriptions.channelName", Message: "must not be empty"}
	}

	return nil
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envInt32(key string, fallback int32) int32 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}

	return int32(parsed)
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func defaultOrigins(environment string) []string {
	if environment == "prod" {
		return nil
	}

	return []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
	}
}

func defaultLogLevel(environment string) string {
	if environment == "local" {
		return "debug"
	}

	return "info"
}

func defaultLogFormat(environment string) string {
	if environment == "prod" {
		return "json"
	}

	return "text"
}
