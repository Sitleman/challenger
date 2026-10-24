package challenge

import (
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "Anna", Day: 1, Reactors: 2},
		{UserID: 2, UserName: "D<i>ma", Day: 1, Reactors: 3},
		{UserID: 1, UserName: "Anna", Day: 2, Reactors: 2},
		{UserID: 2, UserName: "D<i>ma", Day: 3, Reactors: 1},
	}
	now := at(3, 10).Add(33 * time.Minute)

	got := NewBoard(october, proofs, now).Render(now)

	want := "📊 Октябрь 2026 · день 3/31\n" +
		"🦊 Anna — 2/2 · 🔥2\n" +
		"🐻 D&lt;i&gt;ma — 1/2\n" +
		"✅ засчитан · 🟡 ждёт реакций · ❌ пропуск\n" +
		"\n" +
		"<code>     </code>  🦊  🐻\n" +
		"<code>01.10</code>  ✅  ✅\n" +
		"<code>02.10</code>  ✅  ❌\n" +
		"<code>03.10</code>  　  🟡\n" +
		"\n" +
		"обновлено 03.10 в 10:33"
	if got != want {
		t.Fatalf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderWithoutParticipants(t *testing.T) {
	now := at(1, 9)

	got := NewBoard(october, nil, now).Render(now)

	want := "📊 Октябрь 2026 · день 1/31\n" +
		"✅ засчитан · 🟡 ждёт реакций · ❌ пропуск\n" +
		"\n" +
		"Пока никто не отправил #task\n" +
		"\n" +
		"обновлено 01.10 в 09:00"
	if got != want {
		t.Fatalf("Render() =\n%s\nwant\n%s", got, want)
	}
}
