//go:build integration

package persistent_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo/persistent"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:18.2-alpine"
	testDB        = "testdb"
	testUser      = "testuser"
	testPassword  = "testpass"
)

type TranslationRepoSuite struct {
	suite.Suite
	container *tcpg.PostgresContainer
	pg        *postgres.Postgres
	repo      *persistent.TranslationRepo
	ctx       context.Context
}

func (s *TranslationRepoSuite) SetupSuite() {
	s.ctx = context.Background()

	// Start PostgreSQL container.
	container, err := tcpg.Run(s.ctx,
		postgresImage,
		tcpg.WithDatabase(testDB),
		tcpg.WithUsername(testUser),
		tcpg.WithPassword(testPassword),
		tcpg.WithInitScripts(),
		tc.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(s.T(), err)

	s.container = container

	// Get connection string.
	connStr, err := container.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	// Connect to PostgreSQL.
	pg, err := postgres.New(connStr,
		postgres.MaxPoolSize(2),
		postgres.ConnAttempts(5),
		postgres.ConnTimeout(time.Second),
	)
	require.NoError(s.T(), err)

	s.pg = pg

	// Create table schema (apply both migrations).
	_, err = pg.Pool.Exec(s.ctx, `
		CREATE TABLE IF NOT EXISTS history(
			id serial PRIMARY KEY,
			source VARCHAR(255),
			destination VARCHAR(255),
			original VARCHAR(255),
			translation VARCHAR(255)
		);
		ALTER TABLE history ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
		ALTER TABLE history ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
		CREATE INDEX idx_history_created_at ON history(created_at DESC);
	`)
	require.NoError(s.T(), err)

	s.repo = persistent.New(pg)
}

func (s *TranslationRepoSuite) TearDownSuite() {
	if s.pg != nil {
		s.pg.Close()
	}

	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		require.NoError(s.T(), err)
	}
}

func (s *TranslationRepoSuite) SetupTest() {
	// Clean table before each test.
	_, err := s.pg.Pool.Exec(s.ctx, "DELETE FROM history")
	require.NoError(s.T(), err)
}

func (s *TranslationRepoSuite) TestStoreAndGetHistory() {
	t := entity.Translation{
		Source:      "en",
		Destination: "vi",
		Original:    "hello world",
		Translation: "xin chào thế giới",
	}

	// Store.
	err := s.repo.Store(s.ctx, t)
	require.NoError(s.T(), err)

	// Get history.
	history, err := s.repo.GetHistory(s.ctx, 100, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), history, 1)
	require.Equal(s.T(), "hello world", history[0].Original)
	require.Equal(s.T(), "xin chào thế giới", history[0].Translation)
	require.Equal(s.T(), "en", history[0].Source)
	require.Equal(s.T(), "vi", history[0].Destination)
}

func (s *TranslationRepoSuite) TestGetHistoryEmpty() {
	history, err := s.repo.GetHistory(s.ctx, 100, 0)
	require.NoError(s.T(), err)
	require.Empty(s.T(), history)
}

func (s *TranslationRepoSuite) TestGetHistoryPagination() {
	// Insert 5 records.
	for i := range 5 {
		t := entity.Translation{
			Source:      "en",
			Destination: "vi",
			Original:    fmt.Sprintf("text %d", i),
			Translation: fmt.Sprintf("bản dịch %d", i),
		}

		err := s.repo.Store(s.ctx, t)
		require.NoError(s.T(), err)

		// Small sleep to ensure different created_at timestamps.
		time.Sleep(10 * time.Millisecond)
	}

	// Get first 2 items (limit=2, offset=0).
	page1, err := s.repo.GetHistory(s.ctx, 2, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), page1, 2)

	// Should be ordered by created_at DESC — most recent first.
	require.Equal(s.T(), "text 4", page1[0].Original) // most recent
	require.Equal(s.T(), "text 3", page1[1].Original)

	// Get next 2 items (limit=2, offset=2).
	page2, err := s.repo.GetHistory(s.ctx, 2, 2)
	require.NoError(s.T(), err)
	require.Len(s.T(), page2, 2)
	require.Equal(s.T(), "text 2", page2[0].Original)
	require.Equal(s.T(), "text 1", page2[1].Original)

	// Get last page (limit=2, offset=4).
	page3, err := s.repo.GetHistory(s.ctx, 2, 4)
	require.NoError(s.T(), err)
	require.Len(s.T(), page3, 1) // Only 1 item left.
	require.Equal(s.T(), "text 0", page3[0].Original)
}

func (s *TranslationRepoSuite) TestStoreMultipleAndOrder() {
	entries := []entity.Translation{
		{Source: "en", Destination: "vi", Original: "first", Translation: "đầu tiên"},
		{Source: "en", Destination: "vi", Original: "second", Translation: "thứ hai"},
		{Source: "en", Destination: "vi", Original: "third", Translation: "thứ ba"},
	}

	for _, e := range entries {
		err := s.repo.Store(s.ctx, e)
		require.NoError(s.T(), err)

		time.Sleep(10 * time.Millisecond)
	}

	history, err := s.repo.GetHistory(s.ctx, 100, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), history, 3)

	// Most recent entry should be first (created_at DESC).
	require.Equal(s.T(), "third", history[0].Original)
	require.Equal(s.T(), "second", history[1].Original)
	require.Equal(s.T(), "first", history[2].Original)
}

func (s *TranslationRepoSuite) TestGetHistoryNoLimitNoOffset() {
	// Insert 3 records.
	for i := range 3 {
		err := s.repo.Store(s.ctx, entity.Translation{
			Source:      "en",
			Destination: "vi",
			Original:    fmt.Sprintf("item %d", i),
			Translation: fmt.Sprintf("mục %d", i),
		})
		require.NoError(s.T(), err)
	}

	// Limit=0, offset=0 should return all records (no limit applied).
	history, err := s.repo.GetHistory(s.ctx, 0, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), history, 3)
}

func TestTranslationRepoSuite(t *testing.T) {
	suite.Run(t, new(TranslationRepoSuite))
}
