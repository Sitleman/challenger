package challenge

import "time"

const (
	Hashtag     = "#task"
	MinReactors = 2
)

type Status int

const (
	StatusOpen Status = iota
	StatusMissed
	StatusPending
	StatusDone
)

var avatars = []string{
	"🦊", "🐻", "🐼", "🐸", "🐯", "🦁", "🐨", "🐰", "🐱", "🐶",
	"🐵", "🐷", "🐮", "🐔", "🐧", "🦉", "🐙", "🦄", "🐢", "🐳",
}

type Proof struct {
	UserID   int64
	UserName string
	Day      int
	Reactors int
}

type Board struct {
	Month        time.Time
	DaysInMonth  int
	Shown        int
	Today        int
	Participants []Participant
}

type Participant struct {
	Name     string
	Avatar   string
	Statuses []Status
	Score    int
	Closed   int
	Streak   int
}

func NewBoard(month time.Time, proofs []Proof, now time.Time) Board {
	b := Board{Month: month, DaysInMonth: daysIn(month)}
	b.Shown, b.Today = progress(month, b.DaysInMonth, now)
	for i, pp := range collectProofs(proofs) {
		b.Participants = append(b.Participants, newParticipant(i, pp, b.Shown, b.Today))
	}
	return b
}

func daysIn(month time.Time) int {
	return time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, month.Location()).Day()
}

func progress(month time.Time, days int, now time.Time) (shown, today int) {
	now = now.In(month.Location())
	switch {
	case now.Year() == month.Year() && now.Month() == month.Month():
		return now.Day(), now.Day()
	case now.After(month):
		return days, 0
	default:
		return 0, 0
	}
}

type userProofs struct {
	name     string
	reactors map[int]int
}

func collectProofs(proofs []Proof) []*userProofs {
	var ordered []*userProofs
	byUser := map[int64]*userProofs{}
	for _, p := range proofs {
		up, ok := byUser[p.UserID]
		if !ok {
			up = &userProofs{reactors: map[int]int{}}
			byUser[p.UserID] = up
			ordered = append(ordered, up)
		}
		up.name = p.UserName
		if best, seen := up.reactors[p.Day]; !seen || p.Reactors > best {
			up.reactors[p.Day] = p.Reactors
		}
	}
	return ordered
}

func newParticipant(index int, up *userProofs, shown, today int) Participant {
	p := Participant{
		Name:     up.name,
		Avatar:   avatars[index%len(avatars)],
		Statuses: make([]Status, shown),
	}
	for day := 1; day <= shown; day++ {
		s := dayStatus(up.reactors, day, today)
		p.Statuses[day-1] = s
		if isClosed(day, s, today) {
			p.Closed++
			if s == StatusDone {
				p.Score++
			}
		}
	}
	p.Streak = streak(p.Statuses, today)
	return p
}

func dayStatus(reactors map[int]int, day, today int) Status {
	r, ok := reactors[day]
	switch {
	case ok && r >= MinReactors:
		return StatusDone
	case ok:
		return StatusPending
	case day == today:
		return StatusOpen
	default:
		return StatusMissed
	}
}

func isClosed(day int, s Status, today int) bool {
	return day != today || s == StatusDone
}

func streak(statuses []Status, today int) int {
	n := 0
	for day := len(statuses); day >= 1; day-- {
		s := statuses[day-1]
		if !isClosed(day, s, today) {
			continue
		}
		if s != StatusDone {
			break
		}
		n++
	}
	return n
}
