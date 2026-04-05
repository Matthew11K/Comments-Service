package graphql

import (
	"context"
	"net/http"
	"sync"

	"github.com/Matthew11K/Comments-Service/internal/adapters/graphql/graph/model"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type countResolver func(context.Context) (int, error)

type requestState struct {
	loaders                 *Loaders
	postConnectionResolvers map[*model.PostConnection]countResolver
	commentCountResolvers   map[*model.CommentConnection]countResolver
	mu                      sync.RWMutex
}

type requestStateContextKey struct{}

func withRequestState(next http.Handler, service *application.CommentsService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &requestState{
			loaders:                 NewLoaders(service),
			postConnectionResolvers: make(map[*model.PostConnection]countResolver),
			commentCountResolvers:   make(map[*model.CommentConnection]countResolver),
		}
		ctx := context.WithValue(r.Context(), requestStateContextKey{}, state)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestStateFromContext(ctx context.Context) (*requestState, error) {
	state, ok := ctx.Value(requestStateContextKey{}).(*requestState)
	if !ok || state == nil {
		return nil, &domain.OperationError{
			Op: "graphql request state is missing",
		}
	}

	return state, nil
}

func registerPostConnectionResolver(ctx context.Context, obj *model.PostConnection, resolver countResolver) error {
	state, err := requestStateFromContext(ctx)
	if err != nil {
		return err
	}

	state.mu.Lock()
	state.postConnectionResolvers[obj] = resolver
	state.mu.Unlock()
	return nil
}

func registerCommentConnectionResolver(ctx context.Context, obj *model.CommentConnection, resolver countResolver) error {
	state, err := requestStateFromContext(ctx)
	if err != nil {
		return err
	}

	state.mu.Lock()
	state.commentCountResolvers[obj] = resolver
	state.mu.Unlock()
	return nil
}

func resolvePostConnectionCount(ctx context.Context, obj *model.PostConnection) (int, error) {
	state, err := requestStateFromContext(ctx)
	if err != nil {
		return 0, err
	}

	state.mu.RLock()
	resolver, ok := state.postConnectionResolvers[obj]
	state.mu.RUnlock()
	if !ok {
		return 0, &domain.OperationError{
			Op: "post connection resolver is missing",
		}
	}

	return resolver(ctx)
}

func resolveCommentConnectionCount(ctx context.Context, obj *model.CommentConnection) (int, error) {
	state, err := requestStateFromContext(ctx)
	if err != nil {
		return 0, err
	}

	state.mu.RLock()
	resolver, ok := state.commentCountResolvers[obj]
	state.mu.RUnlock()
	if !ok {
		return 0, &domain.OperationError{
			Op: "comment connection resolver is missing",
		}
	}

	return resolver(ctx)
}
