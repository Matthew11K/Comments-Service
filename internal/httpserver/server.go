package httpserver

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/config"
)

type HealthCheck func(context.Context) error

type requestIDContextKey struct{}

func New(
	cfg config.HTTPConfig,
	graphQLPath string,
	graphQLHandler http.Handler,
	playgroundPath string,
	playgroundHandler http.Handler,
	healthCheck HealthCheck,
	logger *slog.Logger,
) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(graphQLPath, graphQLHandler)

	if playgroundHandler != nil {
		mux.Handle(playgroundPath, playgroundHandler)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if healthCheck != nil {
			if err := healthCheck(r.Context()); err != nil {
				logger.WarnContext(r.Context(), "health check failed", "error", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("unhealthy"))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := requestIDMiddleware(loggingMiddleware(mux, logger))

	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

type HijackNotSupportedError struct{}

func (e *HijackNotSupportedError) Error() string {
	return "response writer does not support hijacking"
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, &HijackNotSupportedError{}
	}

	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	flusher.Flush()
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		logger.InfoContext(
			r.Context(),
			"http request completed",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", time.Since(startedAt),
		)
	})
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}
