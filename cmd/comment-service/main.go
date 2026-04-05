package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	memoryevents "github.com/Matthew11K/Comments-Service/internal/adapters/events/memory"
	postgresevents "github.com/Matthew11K/Comments-Service/internal/adapters/events/postgres"
	graphiql "github.com/Matthew11K/Comments-Service/internal/adapters/graphql"
	memoryadapter "github.com/Matthew11K/Comments-Service/internal/adapters/storage/memory"
	postgresadapter "github.com/Matthew11K/Comments-Service/internal/adapters/storage/postgres"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/config"
	"github.com/Matthew11K/Comments-Service/internal/httpserver"
	"github.com/Matthew11K/Comments-Service/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("comment-service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	logger, err := logging.New(cfg.Logging, os.Stdout)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	deps, err := buildDependencies(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer deps.cleanup()

	resolver := graphiql.NewResolver(deps.postsService, deps.commentsService, deps.events, logger)
	graphQLHandler := graphiql.NewHandler(graphiql.ServerConfig{
		MaxPageSize:         cfg.GraphQL.MaxPageSize,
		MaxDepth:            cfg.GraphQL.MaxDepth,
		ComplexityLimit:     cfg.GraphQL.ComplexityLimit,
		QueryCacheSize:      cfg.GraphQL.QueryCacheSize,
		APQCacheSize:        cfg.GraphQL.APQCacheSize,
		ParserTokenLimit:    cfg.GraphQL.ParserTokenLimit,
		EnableIntrospection: cfg.GraphQL.EnableIntrospection,
		AllowedOrigins:      cfg.GraphQL.AllowedOrigins,
		WebsocketKeepAlive:  cfg.GraphQL.WebsocketKeepAlive,
	}, resolver)

	var playgroundHandler http.Handler
	if cfg.GraphQL.EnablePlayground {
		playgroundHandler = graphiql.NewPlaygroundHandler(cfg.GraphQL.Path)
	}

	server := httpserver.New(
		cfg.HTTP,
		cfg.GraphQL.Path,
		graphQLHandler,
		cfg.GraphQL.PlaygroundPath,
		playgroundHandler,
		deps.healthCheck,
		logger,
	)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting http server", "addr", cfg.HTTP.Addr, "backend", cfg.Storage.Backend)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	logger.Info("shutting down http server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	return nil
}

type dependencies struct {
	postsService    *application.PostsService
	commentsService *application.CommentsService
	events          application.CommentEvents
	cleanup         func()
	healthCheck     httpserver.HealthCheck
}

func buildDependencies(ctx context.Context, cfg config.Config, logger *slog.Logger) (dependencies, error) {
	switch cfg.Storage.Backend {
	case "memory":
		store := memoryadapter.NewStore()
		postRepo := memoryadapter.NewPostRepository(store)
		commentRepo := memoryadapter.NewCommentRepository(store)
		txManager := memoryadapter.NewTxManager()
		events := memoryevents.NewBus(cfg.Subscriptions.BufferSize)
		postsService := application.NewPostsService(postRepo, txManager, nil, nil)
		commentsService := application.NewCommentsService(postRepo, commentRepo, txManager, nil, nil, events)
		return dependencies{
			postsService:    postsService,
			commentsService: commentsService,
			events:          events,
			cleanup: func() {
				_ = events.Close()
			},
		}, nil
	case "postgres":
		poolConfig, err := pgxpool.ParseConfig(cfg.Storage.Postgres.DSN)
		if err != nil {
			return dependencies{}, err
		}

		poolConfig.MaxConns = cfg.Storage.Postgres.MaxConns
		poolConfig.MinConns = cfg.Storage.Postgres.MinConns

		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			return dependencies{}, err
		}

		postRepo := postgresadapter.NewPostRepository(pool)
		commentRepo := postgresadapter.NewCommentRepository(pool)
		txManager := postgresadapter.NewTxManager(pool)
		events, err := postgresevents.New(
			ctx,
			cfg.Storage.Postgres.DSN,
			pool,
			cfg.Subscriptions.ChannelName,
			cfg.Subscriptions.BufferSize,
			logger,
		)
		if err != nil {
			pool.Close()
			return dependencies{}, err
		}

		postsService := application.NewPostsService(postRepo, txManager, nil, nil)
		commentsService := application.NewCommentsService(postRepo, commentRepo, txManager, nil, nil, events)
		cleanup := func() {
			_ = events.Close()
			pool.Close()
		}
		healthCheck := func(ctx context.Context) error {
			return pool.Ping(ctx)
		}

		return dependencies{
			postsService:    postsService,
			commentsService: commentsService,
			events:          events,
			cleanup:         cleanup,
			healthCheck:     healthCheck,
		}, nil
	default:
		return dependencies{}, &config.Error{
			Field:   "storage-backend",
			Message: "unsupported backend",
		}
	}
}
