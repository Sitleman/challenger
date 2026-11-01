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

func TestStoppedChallengeRejectsProofAndReactionChanges(t *testing.T) {
	ctx := context.Background()
	q := openTestDB(t)
	c := seedChallenge(t, q, -100)
	seedProof(t, q, c.ID, 42, 7)
	if err := q.AddReaction(ctx, AddReactionParams{ChallengeID: c.ID, MessageID: 42, UserID: 8, UserName: "B"}); err != nil {
		t.Fatal(err)
	}
	stopped, err := q.StopChallenge(ctx, StopChallengeParams{ID: c.ID, StoppedAt: 123})
	if err != nil || stopped.StoppedAt != 123 {
		t.Fatalf("StopChallenge = %+v, %v", stopped, err)
	}
	if _, err := q.StopChallenge(ctx, StopChallengeParams{ID: c.ID, StoppedAt: 456}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second StopChallenge = %v, want sql.ErrNoRows", err)
	}
	if err := q.CreateProof(ctx, CreateProofParams{ChallengeID: c.ID, MessageID: 43, UserID: 9, UserName: "C", Day: 24}); err != nil {
		t.Fatal(err)
	}
	if err := q.AddReaction(ctx, AddReactionParams{ChallengeID: c.ID, MessageID: 42, UserID: 9}); err != nil {
		t.Fatal(err)
	}
	if err := q.RemoveReaction(ctx, RemoveReactionParams{ChallengeID: c.ID, MessageID: 42, UserID: 8}); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListProofStats(ctx, c.ID)
	if err != nil || len(rows) != 1 || rows[0].Reactors != 1 {
		t.Fatalf("frozen stats = %+v, %v", rows, err)
	}
	if _, err := q.FindProof(ctx, FindProofParams{ChatID: c.ChatID, MessageID: 42}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindProof of stopped challenge = %v, want sql.ErrNoRows", err)
	}
}

func TestSupportCountsDistinctOthersAndRecoversLegacyNames(t *testing.T) {
	ctx := context.Background()
	q := openTestDB(t)
	c := seedChallenge(t, q, -100)
	for i, user := range []int64{1, 1, 2, 3, 4} {
		day := int64(24)
		if user == 4 {
			day = 23
		}
		if err := q.CreateProof(ctx, CreateProofParams{ChallengeID: c.ID, MessageID: int64(i + 1), UserID: user, UserName: "Legacy", Day: day}); err != nil {
			t.Fatal(err)
		}
	}
	for _, reaction := range []AddReactionParams{
		{MessageID: 1, UserID: 1},
		{MessageID: 1, UserID: 2}, {MessageID: 2, UserID: 2}, {MessageID: 3, UserID: 2},
		{MessageID: 4, UserID: 2}, {MessageID: 5, UserID: 2},
		{MessageID: 1, UserID: 9, UserName: "Supporter"},
	} {
		reaction.ChallengeID = c.ID
		if err := q.AddReaction(ctx, reaction); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := q.ListSupportStats(ctx, ListSupportStatsParams{ChallengeID: c.ID, EndDay: 24})
	if err != nil || len(rows) != 2 {
		t.Fatalf("ListSupportStats = %+v, %v", rows, err)
	}
	if rows[0].UserID != 2 || rows[0].Recipients != 2 || rows[0].UserName != "Legacy" {
		t.Fatalf("legacy supporter = %+v, want user 2, two recipients and recovered name", rows[0])
	}
	if rows[1].UserID != 9 || rows[1].Recipients != 1 || rows[1].UserName != "Supporter" {
		t.Fatalf("supporter without proofs = %+v", rows[1])
	}
}

func TestLatestChallengeIsScopedToTopicAndMonth(t *testing.T) {
	ctx := context.Background()
	q := openTestDB(t)
	seedChallenge(t, q, -100)
	for _, p := range []CreateChallengeParams{
		{ChatID: -100, ThreadID: 5, Month: "2026-12", StartDay: 1},
		{ChatID: -100, ThreadID: 6, Month: "2026-11", StartDay: 1},
		{ChatID: -200, ThreadID: 5, Month: "2026-11", StartDay: 1},
	} {
		if _, err := q.CreateChallenge(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	c, err := q.GetLatestChallengeByTopic(ctx, GetLatestChallengeByTopicParams{ChatID: -100, ThreadID: 5, Month: "2026-11"})
	if err != nil || c.Month != "2026-10" {
		t.Fatalf("latest challenge = %+v, %v, want October in topic 5", c, err)
	}
}

func TestSummaryMigrationPreservesExistingChallengeData(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 2); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"INSERT INTO challenges (id, chat_id, thread_id, month, stats_message_id, start_day) VALUES (1, -100, 5, '2026-10', 1, 24)",
		"INSERT INTO proofs (challenge_id, message_id, user_id, user_name, day) VALUES (1, 10, 7, 'Anna', 24)",
		"INSERT INTO reactions (challenge_id, message_id, user_id) VALUES (1, 10, 8)",
	} {
		if _, err := conn.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := provider.Up(ctx); err != nil {
			t.Fatal(err)
		}
		q := New(conn)
		c, err := q.GetChallenge(ctx, 1)
		if err != nil || c.StoppedAt != 0 || c.SummaryMessageID != 0 || c.SummaryText != "" || c.SummaryPartsSent != 0 {
			t.Fatalf("migrated challenge = %+v, %v", c, err)
		}
		rows, err := q.ListProofStats(ctx, c.ID)
		if err != nil || len(rows) != 1 || rows[0].Reactors != 1 || rows[0].UserName != "Anna" {
			t.Fatalf("migrated proofs = %+v, %v", rows, err)
		}
		if _, err := provider.DownTo(ctx, 2); err != nil {
			t.Fatal(err)
		}
	}
}
