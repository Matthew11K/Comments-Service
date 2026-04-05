package graphql

import (
	"net/http"
	"slices"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/Matthew11K/Comments-Service/internal/adapters/graphql/generated"
)

type ServerConfig struct {
	MaxPageSize         int
	MaxDepth            int
	ComplexityLimit     int
	QueryCacheSize      int
	APQCacheSize        int
	ParserTokenLimit    int
	EnableIntrospection bool
	AllowedOrigins      []string
	WebsocketKeepAlive  time.Duration
}

func NewHandler(config ServerConfig, resolver *Resolver) http.Handler {
	complexity := generated.ComplexityRoot{}
	complexity.Query.Posts = func(childComplexity int, first int, _ *string) int {
		return 1 + childComplexity*first
	}
	complexity.Post.Comments = func(childComplexity int, first int, _ *string) int {
		return 1 + childComplexity*first
	}
	complexity.Comment.Replies = func(childComplexity int, first int, _ *string) int {
		return 1 + childComplexity*first
	}

	schema := generated.NewExecutableSchema(generated.Config{
		Resolvers:  resolver,
		Complexity: complexity,
	})

	server := handler.New(schema)
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.Websocket{
		KeepAlivePingInterval: config.WebsocketKeepAlive,
		Upgrader: websocket.Upgrader{
			CheckOrigin: originChecker(config.AllowedOrigins),
		},
	})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.POST{})
	server.AddTransport(transport.MultipartForm{})

	queryCacheSize := config.QueryCacheSize
	if queryCacheSize <= 0 {
		queryCacheSize = 1000
	}
	server.SetQueryCache(lru.New[*ast.QueryDocument](queryCacheSize))

	apqCacheSize := config.APQCacheSize
	if apqCacheSize <= 0 {
		apqCacheSize = 100
	}
	server.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](apqCacheSize),
	})

	if config.EnableIntrospection {
		server.Use(extension.Introspection{})
	}

	if config.ComplexityLimit > 0 {
		server.Use(extension.FixedComplexityLimit(config.ComplexityLimit))
	}

	if config.ParserTokenLimit > 0 {
		server.SetParserTokenLimit(config.ParserTokenLimit)
	}

	server.AroundOperations(guardMiddleware(config.MaxDepth, config.MaxPageSize))
	server.SetErrorPresenter(newErrorPresenter(resolver.logger))
	server.SetRecoverFunc(newRecoverFunc(resolver.logger))

	return withActorHeader(withRequestState(server, resolver.comments))
}

func NewPlaygroundHandler(endpoint string) http.Handler {
	return playground.Handler("comment-service GraphQL", endpoint)
}

func originChecker(allowedOrigins []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}

		return slices.Contains(allowedOrigins, origin)
	}
}
