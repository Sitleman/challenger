package challenge

import (
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "Anna (@anna)", Day: 24, Reactors: 2},
		{UserID: 2, UserName: "D<i>ma", Day: 24, Reactors: 3},
		{UserID: 1, UserName: "Anna (@anna)", Day: 25, Reactors: 2},
		{UserID: 2, UserName: "D<i>ma", Day: 26, Reactors: 1},
	}
	now := at(26, 10).Add(33 * time.Minute)

	got := NewBoard(october, 24, proofs, now).Render(now)

	want := "📊 Челлендж · Октябрь 2026\n" +
		"Старт 24.10 · день 3 из 8\n" +
		"\n" +
		"👥 <b>Участники</b>\n" +
		"🦊 Anna (@anna) — 2/2 · 🔥2\n" +
		"🐻 D&lt;i&gt;ma — 1/2\n" +
		"\n" +
		"<code>     </code>  🦊  🐻\n" +
		"<code>24.10</code>  ✅  ✅\n" +
		"<code>25.10</code>  ✅  ❌\n" +
		"<code>26.10</code>  　  🟡\n" +
		"\n" +
		"✅ засчитан · 🟡 ждёт реакций · ❌ пропуск\n" +
		"обновлено 26.10 в 10:33"
	if got != want {
		t.Fatalf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderWithoutParticipants(t *testing.T) {
	now := at(24, 9)

	got := NewBoard(october, 24, nil, now).Render(now)

	want := "📊 Челлендж · Октябрь 2026\n" +
		"Старт 24.10 · день 1 из 8\n" +
		"\n" +
		"👥 <b>Участники</b>\n" +
		"Пока никто не отправил #task\n" +
		"\n" +
		"✅ засчитан · 🟡 ждёт реакций · ❌ пропуск\n" +
		"обновлено 24.10 в 09:00"
	if got != want {
		t.Fatalf("Render() =\n%s\nwant\n%s", got, want)
	}
}
