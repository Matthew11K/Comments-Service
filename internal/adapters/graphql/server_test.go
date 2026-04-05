package graphql

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	memoryevents "github.com/Matthew11K/Comments-Service/internal/adapters/events/memory"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
	applicationmocks "github.com/Matthew11K/Comments-Service/mocks/application"
)

func TestGraphQLRejectsFirstAboveLimit(t *testing.T) {
	t.Parallel()

	postRepo := applicationmocks.NewPostRepository(t)
	handler := newTestGraphQLHandler(t, postRepo, ServerConfig{
		MaxPageSize:         2,
		MaxDepth:            10,
		ComplexityLimit:     100,
		QueryCacheSize:      10,
		APQCacheSize:        10,
		ParserTokenLimit:    1000,
		EnableIntrospection: false,
		AllowedOrigins:      []string{"http://localhost"},
		WebsocketKeepAlive:  time.Second,
	})

	response := executeGraphQL(t, handler, `query { posts(first: 3) { edges { node { id } } pageInfo { hasNextPage } } }`)
	if len(response.Errors) == 0 {
		t.Fatal("expected graphql error")
	}

	if code := response.Errors[0].Extensions["code"]; code != "GRAPHQL_PAGINATION_LIMIT" {
		t.Fatalf("unexpected error code: %v", code)
	}

	postRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
	postRepo.AssertNotCalled(t, "Count", mock.Anything)
}

func TestGraphQLRejectsExcessiveDepth(t *testing.T) {
	t.Parallel()

	postRepo := applicationmocks.NewPostRepository(t)
	handler := newTestGraphQLHandler(t, postRepo, ServerConfig{
		MaxPageSize:         10,
		MaxDepth:            5,
		ComplexityLimit:     100,
		QueryCacheSize:      10,
		APQCacheSize:        10,
		ParserTokenLimit:    1000,
		EnableIntrospection: false,
		AllowedOrigins:      []string{"http://localhost"},
		WebsocketKeepAlive:  time.Second,
	})

	response := executeGraphQL(t, handler, `
		query {
		  posts(first: 1) {
		    edges {
		      node {
		        comments(first: 1) {
		          edges {
		            node {
		              replies(first: 1) {
		                edges {
		                  node {
		                    replies(first: 1) {
		                      edges {
		                        node {
		                          id
		                        }
		                      }
		                    }
		                  }
		                }
		              }
		            }
		          }
		        }
		      }
		    }
		  }
		}
	`)
	if len(response.Errors) == 0 {
		t.Fatal("expected graphql error")
	}

	if code := response.Errors[0].Extensions["code"]; code != "GRAPHQL_DEPTH_LIMIT" {
		t.Fatalf("unexpected error code: %v", code)
	}

	postRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
	postRepo.AssertNotCalled(t, "Count", mock.Anything)
}

func TestGraphQLRejectsComplexQuery(t *testing.T) {
	t.Parallel()

	postRepo := applicationmocks.NewPostRepository(t)
	handler := newTestGraphQLHandler(t, postRepo, ServerConfig{
		MaxPageSize:         100,
		MaxDepth:            10,
		ComplexityLimit:     2,
		QueryCacheSize:      10,
		APQCacheSize:        10,
		ParserTokenLimit:    1000,
		EnableIntrospection: false,
		AllowedOrigins:      []string{"http://localhost"},
		WebsocketKeepAlive:  time.Second,
	})

	response := executeGraphQL(
		t,
		handler,
		`query { posts(first: 5) { edges { node { id title content commentsEnabled } } pageInfo { hasNextPage endCursor } } }`,
	)
	if len(response.Errors) == 0 {
		t.Fatal("expected graphql error")
	}

	if code := response.Errors[0].Extensions["code"]; code != "COMPLEXITY_LIMIT_EXCEEDED" {
		t.Fatalf("unexpected error code: %v", code)
	}

	postRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
	postRepo.AssertNotCalled(t, "Count", mock.Anything)
}

func TestPostsTotalCountIsLazy(t *testing.T) {
	t.Parallel()

	postRepo := applicationmocks.NewPostRepository(t)
	post := mustNewGraphQLPost(t)
	postPage := domain.Page[domain.Post]{
		Items: []domain.Post{post},
	}
	postRepo.EXPECT().List(mock.Anything, mock.Anything).Return(postPage, nil).Twice()
	postRepo.EXPECT().Count(mock.Anything).Return(1, nil).Once()

	handler := newTestGraphQLHandler(t, postRepo, ServerConfig{
		MaxPageSize:         10,
		MaxDepth:            10,
		ComplexityLimit:     100,
		QueryCacheSize:      10,
		APQCacheSize:        10,
		ParserTokenLimit:    1000,
		EnableIntrospection: false,
		AllowedOrigins:      []string{"http://localhost"},
		WebsocketKeepAlive:  time.Second,
	})

	response := executeGraphQL(t, handler, `query { posts(first: 1) { edges { node { id } } pageInfo { hasNextPage } } }`)
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected graphql errors: %+v", response.Errors)
	}

	postRepo.AssertNotCalled(t, "Count", mock.Anything)

	response = executeGraphQL(t, handler, `query { posts(first: 1) { totalCount edges { node { id } } pageInfo { hasNextPage } } }`)
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected graphql errors: %+v", response.Errors)
	}
}

func newTestGraphQLHandler(t *testing.T, postRepo *applicationmocks.PostRepository, cfg ServerConfig) http.Handler {
	t.Helper()

	commentRepo := applicationmocks.NewCommentRepository(t)
	postsService := application.NewPostsService(postRepo, nil, nil, nil)
	commentsService := application.NewCommentsService(postRepo, commentRepo, nil, nil, nil, nil)
	events := memoryevents.NewBus(1)
	t.Cleanup(func() {
		_ = events.Close()
	})

	resolver := NewResolver(postsService, commentsService, events, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewHandler(cfg, resolver)
}

func executeGraphQL(t *testing.T, handler http.Handler, query string) graphqlResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	var response graphqlResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}

	return response
}

type graphqlResponse struct {
	Data   json.RawMessage        `json:"data"`
	Errors []graphqlResponseError `json:"errors"`
}

type graphqlResponseError struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions"`
}

func mustNewGraphQLPost(t *testing.T) domain.Post {
	t.Helper()

	post, err := domain.NewPost(
		domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000001")),
		domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
		"title",
		"content",
		time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return post
}
