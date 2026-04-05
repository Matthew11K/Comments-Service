package postgresitest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	databaseName     = "comment_service"
	databaseUser     = "comment"
	databasePassword = "comment"
	postgresImage    = "postgres:17"
)

type Error struct {
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	switch {
	case e.Message != "":
		return e.Op + ": " + e.Message
	case e.Err != nil:
		return e.Op + ": " + e.Err.Error()
	default:
		return e.Op
	}
}

func (e *Error) Unwrap() error {
	return e.Err
}

type Suite struct {
	Container testcontainers.Container
	DSN       string
	Pool      *pgxpool.Pool
}

func Start(ctx context.Context) (*Suite, error) {
	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: postgresImage,
				Env: map[string]string{
					"POSTGRES_DB":       databaseName,
					"POSTGRES_USER":     databaseUser,
					"POSTGRES_PASSWORD": databasePassword,
				},
				ExposedPorts: []string{"5432/tcp"},
				WaitingFor: wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(2 * time.Minute),
			},
			Started: true,
		},
	)
	if err != nil {
		return nil, &Error{
			Op:  "start postgres test container",
			Err: err,
		}
	}

	suite, err := newSuite(ctx, container)
	if err != nil {
		_ = container.Terminate(context.Background())
		return nil, err
	}

	return suite, nil
}

func newSuite(ctx context.Context, container testcontainers.Container) (*Suite, error) {
	dsn, err := buildDSN(ctx, container)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, &Error{
			Op:  "create postgres test pool",
			Err: err,
		}
	}

	suite := &Suite{
		Container: container,
		DSN:       dsn,
		Pool:      pool,
	}

	if err := suite.waitUntilReady(ctx); err != nil {
		_ = suite.Close(context.Background())
		return nil, err
	}

	if err := suite.applyMigrations(ctx); err != nil {
		_ = suite.Close(context.Background())
		return nil, err
	}

	return suite, nil
}

func buildDSN(ctx context.Context, container testcontainers.Container) (string, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return "", &Error{
			Op:  "resolve postgres test container host",
			Err: err,
		}
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", &Error{
			Op:  "resolve postgres test container port",
			Err: err,
		}
	}

	return "postgres://" + databaseUser + ":" + databasePassword +
		"@" + host + ":" + port.Port() + "/" + databaseName + "?sslmode=disable", nil
}

func (s *Suite) waitUntilReady(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := s.Pool.Ping(ctx); err == nil {
			return nil
		} else if time.Now().After(deadline) {
			return &Error{
				Op:  "ping postgres test database",
				Err: err,
			}
		}

		select {
		case <-ctx.Done():
			return &Error{
				Op:  "wait for postgres test database readiness",
				Err: ctx.Err(),
			}
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (s *Suite) applyMigrations(ctx context.Context) error {
	migrationsPath, err := migrationsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return &Error{
			Op:  "read migrations directory",
			Err: err,
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		body, readErr := os.ReadFile(filepath.Join(migrationsPath, entry.Name()))
		if readErr != nil {
			return &Error{
				Op:  "read migration " + entry.Name(),
				Err: readErr,
			}
		}

		if err := executeStatements(ctx, s.Pool, string(body)); err != nil {
			return &Error{
				Op:  "apply migration " + entry.Name(),
				Err: err,
			}
		}
	}

	return nil
}

func executeStatements(ctx context.Context, pool *pgxpool.Pool, raw string) error {
	for _, statement := range strings.Split(raw, ";") {
		query := strings.TrimSpace(statement)
		if query == "" {
			continue
		}

		if _, err := pool.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (s *Suite) Reset(ctx context.Context) error {
	if _, err := s.Pool.Exec(ctx, `truncate table comments, posts restart identity cascade`); err != nil {
		return &Error{
			Op:  "reset postgres test database",
			Err: err,
		}
	}

	return nil
}

func (s *Suite) Close(ctx context.Context) error {
	if s.Pool != nil {
		s.Pool.Close()
	}

	if s.Container == nil {
		return nil
	}

	if err := s.Container.Terminate(ctx); err != nil {
		return &Error{
			Op:  "terminate postgres test container",
			Err: err,
		}
	}

	return nil
}

func migrationsDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", &Error{
			Op:      "resolve test helper location",
			Message: "runtime caller is unavailable",
		}
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"), nil
}
