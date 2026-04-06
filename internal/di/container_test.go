package di_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Matthew11K/Comments-Service/internal/config"
	"github.com/Matthew11K/Comments-Service/internal/di"
	applicationmocks "github.com/Matthew11K/Comments-Service/mocks/application"
)

func TestNewContainerBuildMemory(t *testing.T) {
	t.Parallel()

	container, err := di.NewContainer(t.Context(), testConfig("memory"), testLogger())
	require.NoError(t, err)

	err = container.Build()
	require.NoError(t, err)

	require.NotNil(t, container.MemoryStore)
	require.NotNil(t, container.TxManager)
	require.NotNil(t, container.Events)
	require.NotNil(t, container.PostsService)
	require.NotNil(t, container.CommentsService)
	require.NotNil(t, container.GraphQLHandler)
	require.NotNil(t, container.HTTPServer)
	require.Len(t, container.Closers, 1)

	require.NoError(t, container.Close(t.Context()))
}

func TestBuildUsesOverridesWithoutInit(t *testing.T) {
	t.Parallel()

	postRepo := applicationmocks.NewPostRepository(t)
	commentRepo := applicationmocks.NewCommentRepository(t)
	txManager := applicationmocks.NewTxManager(t)
	events := applicationmocks.NewCommentEvents(t)

	container := &di.Container{
		Config:            testConfig("postgres"),
		Logger:            testLogger(),
		PostRepository:    postRepo,
		CommentRepository: commentRepo,
		TxManager:         txManager,
		Events:            events,
	}

	err := container.Build()
	require.NoError(t, err)

	require.Nil(t, container.Pool)
	require.NotNil(t, container.PostsService)
	require.NotNil(t, container.CommentsService)
	require.NotNil(t, container.GraphQLHandler)
	require.NotNil(t, container.HTTPServer)
	require.Same(t, postRepo, container.PostRepository)
	require.Same(t, commentRepo, container.CommentRepository)
	require.Same(t, txManager, container.TxManager)
	require.Same(t, events, container.Events)
}

func TestShutdownCallsServerAndClosersInReverseOrder(t *testing.T) {
	t.Parallel()

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: time.Second,
	}
	shutdownCalled := make(chan struct{}, 1)
	server.RegisterOnShutdown(func() {
		shutdownCalled <- struct{}{}
	})

	var (
		mu    sync.Mutex
		order []string
	)
	appendOrder := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	container := &di.Container{
		Config:     testConfig("memory"),
		Logger:     testLogger(),
		HTTPServer: server,
		Closers: []func(context.Context) error{
			func(context.Context) error {
				appendOrder("first")
				return nil
			},
			func(context.Context) error {
				appendOrder("second")
				return nil
			},
		},
	}

	err := container.Shutdown(t.Context())
	require.NoError(t, err)
	<-shutdownCalled

	require.Equal(t, []string{"second", "first"}, order)
}

func TestCloseIsSafeForPartialContainer(t *testing.T) {
	t.Parallel()

	container := &di.Container{
		Config: testConfig("memory"),
		Logger: testLogger(),
	}

	require.NoError(t, container.Close(t.Context()))
	require.NoError(t, container.Close(t.Context()))
}

func testConfig(backend string) config.Config {
	return config.Config{
		Environment: "local",
		HTTP: config.HTTPConfig{
			Addr:            "127.0.0.1:0",
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Storage: config.StorageConfig{
			Backend: backend,
			Postgres: config.PostgresConfig{
				DSN:      "postgres://comment:comment@127.0.0.1:5432/comment?sslmode=disable",
				MaxConns: 4,
				MinConns: 0,
			},
		},
		GraphQL: config.GraphQLConfig{
			Path:                "/query",
			PlaygroundPath:      "/playground",
			EnablePlayground:    true,
			EnableIntrospection: true,
			MaxPageSize:         100,
			MaxDepth:            12,
			ComplexityLimit:     500,
			QueryCacheSize:      1000,
			APQCacheSize:        100,
			ParserTokenLimit:    10000,
			AllowedOrigins:      []string{"http://localhost:8080"},
			WebsocketKeepAlive:  15 * time.Second,
		},
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "text",
		},
		Subscriptions: config.SubscriptionConfig{
			BufferSize:  16,
			ChannelName: "comment_events",
		},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestContainerShutdownAggregatesErrors(t *testing.T) {
	t.Parallel()

	expected := []string{"second", "first"}
	var got []string
	container := &di.Container{
		Config: testConfig("memory"),
		Logger: testLogger(),
		Closers: []func(context.Context) error{
			func(context.Context) error {
				got = append(got, "first")
				return &di.Error{Field: "first", Message: uuid.NewString()}
			},
			func(context.Context) error {
				got = append(got, "second")
				return nil
			},
		},
	}

	err := container.Close(t.Context())
	require.Error(t, err)

	var lifecycleErr *di.LifecycleError
	require.ErrorAs(t, err, &lifecycleErr)
	require.True(t, slices.Equal(expected, got))
}
