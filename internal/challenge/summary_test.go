package challenge

import (
	"strings"
	"testing"
	"time"
)

func TestSummaryRanksDaysAndKeepsAllAwardTies(t *testing.T) {
	proofs := []Proof{
		{MessageID: 10, UserID: 1, UserName: "A<&", Day: 24, Reactors: 4},
		{MessageID: 11, UserID: 1, UserName: "A<&", Day: 24, Reactors: 2},
		{MessageID: 12, UserID: 2, UserName: "B", Day: 24, Reactors: 4},
		{MessageID: 13, UserID: 1, UserName: "A<&", Day: 25, Reactors: 2},
		{MessageID: 14, UserID: 2, UserName: "B", Day: 25, Reactors: 2},
		{MessageID: 15, UserID: 3, UserName: "C", Day: 26, Reactors: 1},
	}
	b := NewFinalBoard(october, 24, proofs, at(26, 12))
	s := Summary{Board: b, Proofs: proofs, ChatID: -100123, Supporters: []Supporter{
		{UserID: 4, Name: "D<&", Recipients: 2},
		{UserID: 5, Name: "E", Recipients: 2},
	}}
	got := s.Render()
	for _, want := range []string{
		"24.10–26.10.2026 · 3 дня · 3 участника",
		"🥇 A&lt;&amp; — 2/3", "🥇 B — 2/3", "3. C — 0/3",
		"Вместе закрыли <b>4 дня</b>", "🟡 Без подтверждения: 1 пруф",
		"🔥 <b>Самая длинная серия</b>", "A&lt;&amp; — 2 дня подряд", "B — 2 дня подряд",
		"❤️ <b>Самый популярный пруф</b>", "https://t.me/c/123/10", "https://t.me/c/123/12",
		"🤝 <b>Главная поддержка</b>", "D&lt;&amp;", "E", "других участников: 2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "🥉") || strings.Contains(got, "https://t.me/c/123/11") {
		t.Fatalf("incorrect rank or non-winning proof:\n%s", got)
	}
}

func TestFinalBoardStopsCalendarAndClosesToday(t *testing.T) {
	proofs := []Proof{{UserID: 1, Day: 24, Reactors: 2}, {UserID: 1, Day: 25, Reactors: 1}}
	b := NewFinalBoard(october, 24, proofs, at(26, 2))
	if b.Shown != 25 || b.Today != 0 || b.Participants[0].Closed != 2 {
		t.Fatalf("final board = %+v, want two closed days ending on 25 October", b)
	}
	got := b.Render(at(29, 12))
	if !strings.Contains(got, "Завершён 25.10") || strings.Contains(got, "29.10") || strings.Contains(got, "ждёт реакций") {
		t.Fatalf("final board is not frozen:\n%s", got)
	}
}

func TestFinalBoardPreviousMonthAndProofBounds(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, Day: 23, Reactors: 2}, {UserID: 2, Day: 31, Reactors: 2},
		{UserID: 3, Day: 32, Reactors: 2},
	}
	b := NewFinalBoard(october, 24, proofs, time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC))
	if b.Shown != 31 || len(b.Participants) != 1 || b.Participants[0].Score != 1 || b.Participants[0].Closed != 8 {
		t.Fatalf("final board = %+v, want only user 2 with 1/8", b)
	}
}

func TestSummaryEmptyAndZeroAwards(t *testing.T) {
	for _, proofs := range [][]Proof{nil, {{MessageID: 1, UserID: 1, UserName: "A", Day: 24}}} {
		got := (Summary{Board: NewFinalBoard(october, 24, proofs, at(24, 12)), Proofs: proofs}).Render()
		if strings.Contains(got, "🥇") || strings.Contains(got, "дней подряд") || strings.Contains(got, "href=\"https://") {
			t.Fatalf("summary invents awards:\n%s", got)
		}
		if proofs == nil && !strings.Contains(got, "Никто не отправил #task") {
			t.Fatalf("empty summary = %s", got)
		}
	}
}

func TestLongestSeriesIsNotJustFinishingSeries(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "A", Day: 24, Reactors: 2},
		{UserID: 1, UserName: "A", Day: 25, Reactors: 2},
		{UserID: 1, UserName: "A", Day: 26, Reactors: 2},
		{UserID: 1, UserName: "A", Day: 28, Reactors: 2},
	}
	got := (Summary{Board: NewFinalBoard(october, 24, proofs, at(28, 12)), Proofs: proofs}).Render()
	if !strings.Contains(got, "A — 3 дня подряд") {
		t.Fatalf("summary = %s, want longest series of 3", got)
	}
}
