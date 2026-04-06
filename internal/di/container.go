package di

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	memoryevents "github.com/Matthew11K/Comments-Service/internal/adapters/events/memory"
	postgresevents "github.com/Matthew11K/Comments-Service/internal/adapters/events/postgres"
	graphqldapter "github.com/Matthew11K/Comments-Service/internal/adapters/graphql"
	memoryadapter "github.com/Matthew11K/Comments-Service/internal/adapters/storage/memory"
	postgresadapter "github.com/Matthew11K/Comments-Service/internal/adapters/storage/postgres"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/config"
	"github.com/Matthew11K/Comments-Service/internal/httpserver"
)

type Container struct {
	Config config.Config
	Logger *slog.Logger

	Pool        *pgxpool.Pool
	MemoryStore *memoryadapter.Store
	HealthCheck httpserver.HealthCheck
	Closers     []func(context.Context) error

	PostRepository    application.PostRepository
	CommentRepository application.CommentRepository
	TxManager         application.TxManager
	Events            application.CommentEvents

	PostsService      *application.PostsService
	CommentsService   *application.CommentsService
	GraphQLHandler    http.Handler
	PlaygroundHandler http.Handler
	HTTPServer        *http.Server

	closed bool
}

func NewContainer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Container, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	container := &Container{
		Config: cfg,
		Logger: withDefaultLogger(logger),
	}

	if err := container.init(ctx); err != nil {
		return nil, err
	}

	return container, nil
}

func (c *Container) init(ctx context.Context) error {
	if err := c.validateBaseDependencies(); err != nil {
		return err
	}

	switch c.Config.Storage.Backend {
	case "memory":
		return c.initMemory()
	case "postgres":
		return c.initPostgres(ctx)
	default:
		return &InitError{
			Component: "storage backend",
			Err: &config.Error{
				Field:   "storage-backend",
				Message: "unsupported backend",
			},
		}
	}
}

func (c *Container) Build() error {
	c.Logger = withDefaultLogger(c.Logger)

	if err := c.validateBaseDependencies(); err != nil {
		return err
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{name: "repositories", fn: c.buildRepositories},
		{name: "services", fn: c.buildServices},
		{name: "graphql", fn: c.buildGraphQL},
		{name: "http", fn: c.buildHTTP},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			return &BuildError{
				Component: step.name,
				Err:       err,
			}
		}
	}

	return nil
}

func (c *Container) Close(ctx context.Context) error {
	if c.closed {
		return nil
	}

	c.closed = true
	var errs []error

	for idx := len(c.Closers) - 1; idx >= 0; idx-- {
		if c.Closers[idx] == nil {
			continue
		}

		if err := c.Closers[idx](ctx); err != nil {
			errs = append(errs, err)
		}
	}

	c.Closers = nil

	if len(errs) == 0 {
		return nil
	}

	return &LifecycleError{
		Operation: "close container",
		Errors:    errs,
	}
}

func (c *Container) Shutdown(ctx context.Context) error {
	var errs []error

	if c.HTTPServer != nil {
		if err := c.HTTPServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		c.HTTPServer = nil
	}

	if err := c.Close(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return &LifecycleError{
		Operation: "shutdown container",
		Errors:    errs,
	}
}

func (c *Container) initMemory() error {
	store := memoryadapter.NewStore()
	events := memoryevents.NewBus(c.Config.Subscriptions.BufferSize)

	c.MemoryStore = store
	if c.TxManager == nil {
		c.TxManager = memoryadapter.NewTxManager()
	}
	if c.Events == nil {
		c.Events = events
	}
	c.Closers = append(c.Closers, func(context.Context) error {
		return events.Close()
	})

	return nil
}

func (c *Container) initPostgres(ctx context.Context) error {
	poolConfig, err := pgxpool.ParseConfig(c.Config.Storage.Postgres.DSN)
	if err != nil {
		return &InitError{
			Component: "postgres pool config",
			Err:       err,
		}
	}

	poolConfig.MaxConns = c.Config.Storage.Postgres.MaxConns
	poolConfig.MinConns = c.Config.Storage.Postgres.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return &InitError{
			Component: "postgres pool",
			Err:       err,
		}
	}

	c.Pool = pool
	c.HealthCheck = func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
	c.Closers = append(c.Closers, func(context.Context) error {
		pool.Close()
		return nil
	})

	if c.TxManager == nil {
		c.TxManager = postgresadapter.NewTxManager(pool)
	}

	events, err := postgresevents.New(
		ctx,
		c.Config.Storage.Postgres.DSN,
		pool,
		c.Config.Subscriptions.ChannelName,
		c.Config.Subscriptions.BufferSize,
		c.Logger,
	)
	if err != nil {
		pool.Close()
		c.Pool = nil
		c.Closers = c.Closers[:len(c.Closers)-1]
		return &InitError{
			Component: "postgres events",
			Err:       err,
		}
	}

	if c.Events == nil {
		c.Events = events
	}
	c.Closers = append(c.Closers, func(context.Context) error {
		return events.Close()
	})

	return nil
}

func (c *Container) buildRepositories() error {
	if c.PostRepository != nil && c.CommentRepository != nil {
		return nil
	}

	switch c.Config.Storage.Backend {
	case "memory":
		if c.MemoryStore == nil {
			return &MissingDependencyError{Dependency: "memory store"}
		}

		if c.PostRepository == nil {
			c.PostRepository = memoryadapter.NewPostRepository(c.MemoryStore)
		}
		if c.CommentRepository == nil {
			c.CommentRepository = memoryadapter.NewCommentRepository(c.MemoryStore)
		}
	case "postgres":
		if c.Pool == nil {
			return &MissingDependencyError{Dependency: "postgres pool"}
		}

		if c.PostRepository == nil {
			c.PostRepository = postgresadapter.NewPostRepository(c.Pool)
		}
		if c.CommentRepository == nil {
			c.CommentRepository = postgresadapter.NewCommentRepository(c.Pool)
		}
	default:
		if c.PostRepository == nil {
			return &MissingDependencyError{Dependency: "post repository"}
		}
		if c.CommentRepository == nil {
			return &MissingDependencyError{Dependency: "comment repository"}
		}
	}

	return nil
}

func (c *Container) buildServices() error {
	if c.PostsService == nil {
		if c.PostRepository == nil {
			return &MissingDependencyError{Dependency: "post repository"}
		}
		if c.TxManager == nil {
			return &MissingDependencyError{Dependency: "tx manager"}
		}

		c.PostsService = application.NewPostsService(c.PostRepository, c.TxManager, nil, nil)
	}

	if c.CommentsService == nil {
		if c.PostRepository == nil {
			return &MissingDependencyError{Dependency: "post repository"}
		}
		if c.CommentRepository == nil {
			return &MissingDependencyError{Dependency: "comment repository"}
		}
		if c.TxManager == nil {
			return &MissingDependencyError{Dependency: "tx manager"}
		}
		if c.Events == nil {
			return &MissingDependencyError{Dependency: "comment events"}
		}

		c.CommentsService = application.NewCommentsService(
			c.PostRepository,
			c.CommentRepository,
			c.TxManager,
			nil,
			nil,
			c.Events,
		)
	}

	return nil
}

func (c *Container) buildGraphQL() error {
	if c.GraphQLHandler != nil {
		return nil
	}

	if c.PostsService == nil {
		return &MissingDependencyError{Dependency: "posts service"}
	}
	if c.CommentsService == nil {
		return &MissingDependencyError{Dependency: "comments service"}
	}
	if c.Events == nil {
		return &MissingDependencyError{Dependency: "comment events"}
	}

	resolver := graphqldapter.NewResolver(c.PostsService, c.CommentsService, c.Events, c.Logger)
	c.GraphQLHandler = graphqldapter.NewHandler(graphqldapter.ServerConfig{
		MaxPageSize:         c.Config.GraphQL.MaxPageSize,
		MaxDepth:            c.Config.GraphQL.MaxDepth,
		ComplexityLimit:     c.Config.GraphQL.ComplexityLimit,
		QueryCacheSize:      c.Config.GraphQL.QueryCacheSize,
		APQCacheSize:        c.Config.GraphQL.APQCacheSize,
		ParserTokenLimit:    c.Config.GraphQL.ParserTokenLimit,
		EnableIntrospection: c.Config.GraphQL.EnableIntrospection,
		AllowedOrigins:      c.Config.GraphQL.AllowedOrigins,
		WebsocketKeepAlive:  c.Config.GraphQL.WebsocketKeepAlive,
	}, resolver)

	return nil
}

func (c *Container) buildHTTP() error {
	if c.HTTPServer != nil {
		return nil
	}
	if c.GraphQLHandler == nil {
		return &MissingDependencyError{Dependency: "graphql handler"}
	}

	if c.PlaygroundHandler == nil && c.Config.GraphQL.EnablePlayground {
		c.PlaygroundHandler = graphqldapter.NewPlaygroundHandler(c.Config.GraphQL.Path)
	}

	c.HTTPServer = httpserver.New(
		c.Config.HTTP,
		c.Config.GraphQL.Path,
		c.GraphQLHandler,
		c.Config.GraphQL.PlaygroundPath,
		c.PlaygroundHandler,
		c.HealthCheck,
		c.Logger,
	)

	return nil
}

func (c *Container) validateBaseDependencies() error {
	if err := c.Config.Validate(); err != nil {
		return &Error{
			Field:   "config",
			Message: err.Error(),
		}
	}

	return nil
}

func withDefaultLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}

	return slog.Default()
}
