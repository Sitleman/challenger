package challenge

import (
	"strings"
	"testing"
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
	b := NewFinalBoard(october, 24, 26, proofs)
	s := Summary{Board: b, Proofs: proofs, ChatID: -100123}
	got := renderSummary(t, s)
	for _, want := range []string{
		"Октябрьский челлендж завершён!", "24–26 октября · 3 участника",
		`🥇 <a href="tg://user?id=1">A&lt;&amp;</a> — 2/3`, `🥇 <a href="tg://user?id=2">B</a> — 2/3`,
		`<a href="tg://user?id=3">C</a> — 0/3`,
		"Вместе — <b>4 из 9 возможных дней</b>", "🟡 Без подтверждения: 1 пруф",
		"🔥 <b>Самая длинная серия — 2 дня</b>",
		"❤️ <b>Популярные пруфы — по 4 реакции</b>", `href="https://t.me/c/123/10">24.10</a>`, "https://t.me/c/123/12",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "🥉") || strings.Contains(got, "поддержка") || strings.Contains(got, "3. ") || strings.Contains(got, "https://t.me/c/123/11") {
		t.Fatalf("incorrect rank or non-winning proof:\n%s", got)
	}
}

func TestFinalBoardStopsCalendarAndClosesToday(t *testing.T) {
	proofs := []Proof{{UserID: 1, Day: 24, Reactors: 2}, {UserID: 1, Day: 25, Reactors: 1}}
	b := NewFinalBoard(october, 24, 25, proofs)
	if b.Shown != 25 || b.Today != 0 || b.Participants[0].Closed != 2 {
		t.Fatalf("final board = %+v, want two closed days ending on 25 October", b)
	}
	got := renderBoard(t, b, at(29, 12))
	if !strings.Contains(got, "Завершён 25.10") || strings.Contains(got, "29.10") || strings.Contains(got, "ждёт реакций") {
		t.Fatalf("final board is not frozen:\n%s", got)
	}
}

func TestFinalBoardPreviousMonthAndProofBounds(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, Day: 23, Reactors: 2}, {UserID: 2, Day: 31, Reactors: 2},
		{UserID: 3, Day: 32, Reactors: 2},
	}
	b := NewFinalBoard(october, 24, 32, proofs)
	if b.Shown != 31 || len(b.Participants) != 1 || b.Participants[0].Score != 1 || b.Participants[0].Closed != 8 {
		t.Fatalf("final board = %+v, want only user 2 with 1/8", b)
	}
}

func TestSummaryEmptyAndZeroAwards(t *testing.T) {
	for _, proofs := range [][]Proof{nil, {{MessageID: 1, UserID: 1, UserName: "A", Day: 24}}} {
		got := renderSummary(t, Summary{Board: NewFinalBoard(october, 24, 24, proofs), Proofs: proofs})
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
	got := renderSummary(t, Summary{Board: NewFinalBoard(october, 24, 28, proofs), Proofs: proofs})
	if !strings.Contains(got, "Самая длинная серия — 3 дня") {
		t.Fatalf("summary = %s, want longest series of 3", got)
	}
}

func renderSummary(t *testing.T, s Summary) string {
	t.Helper()
	text, err := s.Render()
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestSummaryFitsOneMessageWithLongNamesAndManyTies(t *testing.T) {
	var proofs []Proof
	for i := int64(1); i <= 100; i++ {
		name := strings.Repeat("🦊<&", 100)
		proofs = append(proofs, Proof{MessageID: i, UserID: i, UserName: name, Day: 24, Reactors: 2})
	}
	s := Summary{Board: NewFinalBoard(october, 24, 26, proofs), Proofs: proofs, ChatID: -100123, StatsMessageID: 1}
	got := renderSummary(t, s)
	if n := messageLength(got); n > 1024 {
		t.Fatalf("summary length = %d, want <= 1024", n)
	}
	for _, want := range []string{"Результаты", "Самая длинная серия", "Популярные пруфы", "…ещё", "https://t.me/c/123/1"} {
		if !strings.Contains(got, want) {
			t.Errorf("compact summary missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "ZgotmplZ") || strings.Count(got, "<a ") != strings.Count(got, "</a>") {
		t.Fatalf("invalid Telegram links: %s", got)
	}
}

func TestSummaryTemplatesEscapeNamesAndKeepTelegramMentions(t *testing.T) {
	name := "<b>A & B</b>\n<script>x</script>"
	proofs := []Proof{{MessageID: 1, UserID: 1, UserName: name, Day: 24, Reactors: 2}}
	got := renderSummary(t, Summary{
		Board: NewFinalBoard(october, 24, 24, proofs), Proofs: proofs,
	})
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>A") || !strings.Contains(got, "&lt;b&gt;A &amp; B&lt;/b&gt;") {
		t.Fatalf("user-supplied markup was not escaped: %s", got)
	}
	if !strings.Contains(got, `href="tg://user?id=1"`) || strings.Contains(got, "ZgotmplZ") {
		t.Fatalf("Telegram mention was filtered out: %s", got)
	}
}

func TestSummaryGroupsTiedProofsByUserAndKeepsDatedLinks(t *testing.T) {
	proofs := []Proof{
		{MessageID: 1, UserID: 1, UserName: "Anna (@anna)", Day: 25, Reactors: 4},
		{MessageID: 2, UserID: 2, UserName: "Anna (@other)", Day: 24, Reactors: 4},
		{MessageID: 3, UserID: 1, UserName: "Anna (@anna)", Day: 24, Reactors: 4},
		{MessageID: 4, UserID: 3, UserName: "Bob (@bob)", Day: 24, Reactors: 4},
		{MessageID: 5, UserID: 1, UserName: "Anna (@anna)", Day: 23, Reactors: 9},
	}
	got := renderSummary(t, Summary{Board: NewFinalBoard(october, 24, 25, proofs), Proofs: proofs, ChatID: -100123})
	for _, want := range []string{
		`Anna (@anna)</a> — <a href="https://t.me/c/123/3">24.10</a> · <a href="https://t.me/c/123/1">25.10</a>`,
		`Anna (@other)</a> — <a href="https://t.me/c/123/2">24.10</a>`,
		`Bob</a> — <a href="https://t.me/c/123/4">24.10</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "https://t.me/c/123/5") || strings.Contains(got, "@bob") {
		t.Fatalf("unexpected proof or username: %s", got)
	}
}

func TestSummaryBoundsManyTiedProofsFromOneAuthor(t *testing.T) {
	var proofs []Proof
	for i := int64(1); i <= 1000; i++ {
		proofs = append(proofs, Proof{MessageID: i, UserID: 1, UserName: "A", Day: 24, Reactors: 2})
	}
	got := renderSummary(t, Summary{Board: NewFinalBoard(october, 24, 24, proofs), Proofs: proofs, ChatID: -100123})
	if messageLength(got) > summaryMaxLength || !strings.Contains(got, "…ещё") {
		t.Fatalf("unbounded proofs: %s", got)
	}
}
