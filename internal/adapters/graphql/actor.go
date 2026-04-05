package graphql

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

const actorHeader = "X-Actor-ID"

type actorState struct {
	actor domain.UserID
	err   error
	set   bool
}

type actorContextKey struct{}

func withActorHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawActorID := r.Header.Get(actorHeader)
		if rawActorID == "" {
			next.ServeHTTP(w, r)
			return
		}

		parsedActorID, err := uuid.Parse(rawActorID)
		state := actorState{
			set: true,
			err: err,
		}
		if err == nil {
			state.actor = domain.NewUserID(parsedActorID)
		}

		ctx := context.WithValue(r.Context(), actorContextKey{}, state)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func actorFromContext(ctx context.Context) (domain.UserID, error) {
	state, ok := ctx.Value(actorContextKey{}).(actorState)
	if !ok || !state.set {
		return domain.UserID{}, &domain.ValidationError{
			Field:   actorHeader,
			Message: "header is required",
		}
	}

	if state.err != nil {
		return domain.UserID{}, &domain.ValidationError{
			Field:   actorHeader,
			Message: "must be a valid UUID",
		}
	}

	return state.actor, nil
}
