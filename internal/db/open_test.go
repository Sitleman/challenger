package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Sitleman/challenger/migrations"
	"github.com/pressly/goose/v3"
)

func openTestDB(t *testing.T) *Queries {
	t.Helper()
	conn, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return New(conn)
}

func TestMigrationsRoundTrip(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations.FS)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestFindProofByChatAndMessage(t *testing.T) {
	ctx := context.Background()
	q := openTestDB(t)
	c := seedChallenge(t, q, -100)
	seedProof(t, q, c.ID, 42, 7)

	p, err := q.FindProof(ctx, FindProofParams{ChatID: -100, MessageID: 42})
	if err != nil {
		t.Fatalf("FindProof: %v", err)
	}
	if p.UserID != 7 {
		t.Fatalf("UserID = %d, want 7", p.UserID)
	}

	_, err = q.FindProof(ctx, FindProofParams{ChatID: -200, MessageID: 42})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindProof in other chat: err = %v, want sql.ErrNoRows", err)
	}
}

func TestReactionsAreCountedOncePerUser(t *testing.T) {
	ctx := context.Background()
	q := openTestDB(t)
	c := seedChallenge(t, q, -100)
	seedProof(t, q, c.ID, 42, 7)

	for _, user := range []int64{1, 1, 2} {
		if err := q.AddReaction(ctx, AddReactionParams{ChallengeID: c.ID, MessageID: 42, UserID: user}); err != nil {
			t.Fatalf("AddReaction: %v", err)
		}
	}
	if got := reactors(t, q, c.ID); got != 2 {
		t.Fatalf("reactors after adds = %d, want 2", got)
	}

	if err := q.RemoveReaction(ctx, RemoveReactionParams{ChallengeID: c.ID, MessageID: 42, UserID: 1}); err != nil {
		t.Fatalf("RemoveReaction: %v", err)
	}
	if got := reactors(t, q, c.ID); got != 1 {
		t.Fatalf("reactors after remove = %d, want 1", got)
	}
}

func seedChallenge(t *testing.T, q *Queries, chatID int64) Challenge {
	t.Helper()
	c, err := q.CreateChallenge(context.Background(), CreateChallengeParams{
		ChatID: chatID, ThreadID: 5, Month: "2026-10", StatsMessageID: 1,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	return c
}

func seedProof(t *testing.T, q *Queries, challengeID, messageID, userID int64) {
	t.Helper()
	err := q.CreateProof(context.Background(), CreateProofParams{
		ChallengeID: challengeID, MessageID: messageID, UserID: userID, UserName: "Anna", Day: 1,
	})
	if err != nil {
		t.Fatalf("CreateProof: %v", err)
	}
}

func reactors(t *testing.T, q *Queries, challengeID int64) int64 {
	t.Helper()
	rows, err := q.ListProofStats(context.Background(), challengeID)
	if err != nil {
		t.Fatalf("ListProofStats: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListProofStats rows = %d, want 1", len(rows))
	}
	return rows[0].Reactors
}
