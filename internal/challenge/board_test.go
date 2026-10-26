package challenge

import (
	"reflect"
	"testing"
	"time"
)

var october = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

func at(day, hour int) time.Time {
	return time.Date(2026, time.October, day, hour, 0, 0, 0, time.UTC)
}

func TestNewBoardDayStatuses(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "Anna", Day: 1, Reactors: 2},
		{UserID: 1, UserName: "Anna", Day: 2, Reactors: 1},
	}

	b := NewBoard(october, 1, proofs, at(4, 12))

	want := []Status{StatusDone, StatusPending, StatusMissed, StatusOpen}
	if got := b.Participants[0].Statuses; !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
}

func TestNewBoardTakesBestProofOfTheDay(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "Anna", Day: 1, Reactors: 3},
		{UserID: 1, UserName: "Anna", Day: 1, Reactors: 0},
	}

	b := NewBoard(october, 1, proofs, at(1, 12))

	if got := b.Participants[0].Statuses[0]; got != StatusDone {
		t.Fatalf("status = %v, want %v", got, StatusDone)
	}
}

func TestNewBoardScoreCountsOnlyClosedDays(t *testing.T) {
	tests := []struct {
		name       string
		proofs     []Proof
		wantScore  int
		wantClosed int
	}{
		{
			name:       "open today is not counted",
			proofs:     []Proof{{UserID: 1, Day: 1, Reactors: 2}},
			wantScore:  1,
			wantClosed: 2,
		},
		{
			name:       "pending today is not counted",
			proofs:     []Proof{{UserID: 1, Day: 1, Reactors: 2}, {UserID: 1, Day: 3, Reactors: 1}},
			wantScore:  1,
			wantClosed: 2,
		},
		{
			name:       "done today is counted",
			proofs:     []Proof{{UserID: 1, Day: 1, Reactors: 2}, {UserID: 1, Day: 3, Reactors: 2}},
			wantScore:  2,
			wantClosed: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewBoard(october, 1, tt.proofs, at(3, 12)).Participants[0]

			if p.Score != tt.wantScore || p.Closed != tt.wantClosed {
				t.Fatalf("score = %d/%d, want %d/%d", p.Score, p.Closed, tt.wantScore, tt.wantClosed)
			}
		})
	}
}

func TestNewBoardStreak(t *testing.T) {
	tests := []struct {
		name   string
		proofs []Proof
		want   int
	}{
		{
			name:   "open today keeps streak",
			proofs: []Proof{{UserID: 1, Day: 2, Reactors: 2}, {UserID: 1, Day: 3, Reactors: 2}},
			want:   2,
		},
		{
			name:   "done today extends streak",
			proofs: []Proof{{UserID: 1, Day: 3, Reactors: 2}, {UserID: 1, Day: 4, Reactors: 2}},
			want:   2,
		},
		{
			name:   "missed day breaks streak",
			proofs: []Proof{{UserID: 1, Day: 1, Reactors: 2}, {UserID: 1, Day: 3, Reactors: 2}},
			want:   1,
		},
		{
			name:   "pending past day breaks streak",
			proofs: []Proof{{UserID: 1, Day: 2, Reactors: 2}, {UserID: 1, Day: 3, Reactors: 1}},
			want:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewBoard(october, 1, tt.proofs, at(4, 12)).Participants[0]

			if p.Streak != tt.want {
				t.Fatalf("streak = %d, want %d", p.Streak, tt.want)
			}
		})
	}
}

func TestNewBoardParticipantsOrderedByFirstProof(t *testing.T) {
	proofs := []Proof{
		{UserID: 2, UserName: "Dima", Day: 1},
		{UserID: 1, UserName: "Anna", Day: 1},
		{UserID: 2, UserName: "Dmitry", Day: 2},
	}

	b := NewBoard(october, 1, proofs, at(2, 12))

	got := []string{b.Participants[0].Name, b.Participants[1].Name}
	if want := []string{"Dmitry", "Anna"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	if b.Participants[0].Avatar != avatars[0] || b.Participants[1].Avatar != avatars[1] {
		t.Fatalf("avatars = %q %q, want %q %q", b.Participants[0].Avatar, b.Participants[1].Avatar, avatars[0], avatars[1])
	}
}

func TestNewBoardFinishedMonthShowsAllDaysClosed(t *testing.T) {
	proofs := []Proof{{UserID: 1, Day: 31, Reactors: 1}}

	b := NewBoard(october, 1, proofs, time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC))

	if b.Shown != 31 || b.Today != 0 {
		t.Fatalf("shown/today = %d/%d, want 31/0", b.Shown, b.Today)
	}
	if p := b.Participants[0]; p.Closed != 31 || p.Statuses[30] != StatusPending {
		t.Fatalf("closed = %d, last status = %v, want 31 and pending", p.Closed, p.Statuses[30])
	}
}

func TestNewBoardCountsFromStartDay(t *testing.T) {
	proofs := []Proof{
		{UserID: 1, UserName: "Early", Day: 20, Reactors: 2},
		{UserID: 2, UserName: "Anna", Day: 23, Reactors: 2},
		{UserID: 2, UserName: "Anna", Day: 24, Reactors: 2},
	}

	b := NewBoard(october, 24, proofs, at(25, 12))

	if len(b.Participants) != 1 || b.Participants[0].Name != "Anna" {
		t.Fatalf("participants = %+v, want only Anna", b.Participants)
	}
	p := b.Participants[0]
	if want := []Status{StatusDone, StatusOpen}; !reflect.DeepEqual(p.Statuses, want) {
		t.Fatalf("statuses = %v, want %v", p.Statuses, want)
	}
	if p.Score != 1 || p.Closed != 1 || p.Streak != 1 {
		t.Fatalf("score = %d/%d streak %d, want 1/1 streak 1", p.Score, p.Closed, p.Streak)
	}
}

func TestDayOf(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "before day start", at: time.Date(2026, time.October, 25, 2, 59, 0, 0, msk), want: "2026-10-24"},
		{name: "at day start", at: time.Date(2026, time.October, 25, 3, 0, 0, 0, msk), want: "2026-10-25"},
		{name: "evening", at: time.Date(2026, time.October, 25, 23, 59, 0, 0, msk), want: "2026-10-25"},
		{name: "night of new month", at: time.Date(2026, time.November, 1, 1, 0, 0, 0, msk), want: "2026-10-31"},
		{name: "converts to location", at: time.Date(2026, time.October, 25, 0, 30, 0, 0, time.UTC), want: "2026-10-25"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DayOf(tt.at, msk).Format(time.DateOnly); got != tt.want {
				t.Fatalf("DayOf = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDayOfFollowsWallClockAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no tzdata: %v", err)
	}
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{name: "spring forward after day start", at: time.Date(2026, time.March, 29, 1, 30, 0, 0, time.UTC), want: "2026-03-29"},
		{name: "fall back before day start", at: time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC), want: "2026-10-24"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DayOf(tt.at, berlin).Format(time.DateOnly); got != tt.want {
				t.Fatalf("DayOf = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewBoardTodayStartsAtDayStartHour(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	month := time.Date(2026, time.October, 1, 0, 0, 0, 0, msk)
	tests := []struct {
		name      string
		now       time.Time
		wantShown int
		wantToday int
	}{
		{name: "night before day start", now: time.Date(2026, time.October, 25, 2, 59, 0, 0, msk), wantShown: 24, wantToday: 24},
		{name: "at day start", now: time.Date(2026, time.October, 25, 3, 0, 0, 0, msk), wantShown: 25, wantToday: 25},
		{name: "night of next month", now: time.Date(2026, time.November, 1, 1, 0, 0, 0, msk), wantShown: 31, wantToday: 31},
		{name: "next month after day start", now: time.Date(2026, time.November, 1, 3, 0, 0, 0, msk), wantShown: 31, wantToday: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBoard(month, 1, nil, tt.now)
			if b.Shown != tt.wantShown || b.Today != tt.wantToday {
				t.Fatalf("shown/today = %d/%d, want %d/%d", b.Shown, b.Today, tt.wantShown, tt.wantToday)
			}
		})
	}
}
