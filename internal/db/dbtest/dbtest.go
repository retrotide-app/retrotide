package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImageName = "postgres:18"
	postgresUser      = "retrotide"
	postgresPassword  = "retrotide_test"
	postgresAdminDB   = "postgres"
)

var (
	EnsureRunning = sync.OnceValue(startContainer)

	adminDSN string

	dbCounter   int
	dbCounterMu sync.Mutex
)

func startContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		postgresImageName,
		tcpostgres.WithDatabase(postgresAdminDB),
		tcpostgres.WithUsername(postgresUser),
		tcpostgres.WithPassword(postgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("start postgres container: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("get admin connection string: %w", err)
	}

	adminDSN = dsn

	return nil
}

func AdminDSN(t *testing.T) string {
	t.Helper()

	require.NoError(t, EnsureRunning())

	return adminDSN
}

func nextDBName() string {
	dbCounterMu.Lock()
	defer dbCounterMu.Unlock()

	dbCounter++

	return fmt.Sprintf("test_%d_%d", os.Getpid(), dbCounter)
}

func execAdmin(ctx context.Context, statement string) (err error) {
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("open admin: %w", err)
	}

	defer func() {
		if closeErr := admin.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close admin: %w", closeErr))
		}
	}()

	if _, err = admin.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("exec %q: %w", statement, err)
	}

	return nil
}

func DSNFor(dbName string) string {
	prefix, rest, found := strings.CutLast(adminDSN, "/")
	if !found {
		return adminDSN
	}

	_, query, hasQuery := strings.Cut(rest, "?")
	if !hasQuery {
		return prefix + "/" + dbName
	}

	return prefix + "/" + dbName + "?" + query
}

func DSNForDatabase(t *testing.T, dbName string) string {
	t.Helper()

	require.NoError(t, EnsureRunning())

	return DSNFor(dbName)
}

func openWithCleanup(t *testing.T, dbName string) *sql.DB {
	t.Helper()

	database, err := sql.Open("pgx", DSNForDatabase(t, dbName))
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close test database %s: %v", dbName, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := execAdmin(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, dbName)); err != nil {
			t.Errorf("drop test database %s: %v", dbName, err)
		}
	})

	return database
}

func NewEmptyDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()

	require.NoError(t, EnsureRunning())

	dbName := nextDBName()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	require.NoError(t, CreateDatabase(ctx, dbName))

	return openWithCleanup(t, dbName), dbName
}

func NewDatabaseFromTemplate(t *testing.T, templateName string) *sql.DB {
	t.Helper()

	require.NoError(t, EnsureRunning())

	dbName := nextDBName()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	require.NoError(t, execAdmin(ctx, fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, dbName, templateName)))

	return openWithCleanup(t, dbName)
}

func MarkAsTemplate(ctx context.Context, dbName string) error {
	if err := execAdmin(ctx, fmt.Sprintf(`UPDATE pg_database SET datistemplate = TRUE WHERE datname = '%s'`, dbName)); err != nil {
		return fmt.Errorf("mark template: %w", err)
	}

	return nil
}

func CreateDatabase(ctx context.Context, dbName string) error {
	if err := execAdmin(ctx, fmt.Sprintf(`CREATE DATABASE %s`, dbName)); err != nil {
		return fmt.Errorf("create db: %w", err)
	}

	return nil
}
