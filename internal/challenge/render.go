package challenge

import (
	"fmt"
	"html"
	"strings"
	"time"
)

const (
	cellGap   = "  "
	emptyCell = "　"
	legend    = "✅ засчитан · 🟡 ждёт реакций · ❌ пропуск"
)

var monthNames = []string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

var cells = map[Status]string{
	StatusOpen:    emptyCell,
	StatusMissed:  "❌",
	StatusPending: "🟡",
	StatusDone:    "✅",
}

func (b Board) Render(now time.Time) string {
	lines := []string{b.title()}
	for _, p := range b.Participants {
		lines = append(lines, p.summary())
	}
	lines = append(lines, legend, "")
	lines = append(lines, b.table()...)
	lines = append(lines, "", updatedAt(now.In(b.Month.Location())))
	return strings.Join(lines, "\n")
}

func (b Board) title() string {
	return fmt.Sprintf("📊 %s %d · день %d/%d", monthNames[b.Month.Month()-1], b.Month.Year(), b.Shown, b.DaysInMonth)
}

func (p Participant) summary() string {
	s := fmt.Sprintf("%s %s — %d/%d", p.Avatar, html.EscapeString(p.Name), p.Score, p.Closed)
	if p.Streak > 0 {
		s += fmt.Sprintf(" · 🔥%d", p.Streak)
	}
	return s
}

func (b Board) table() []string {
	if len(b.Participants) == 0 {
		return []string{"Пока никто не отправил " + Hashtag}
	}
	header := code("     ")
	for _, p := range b.Participants {
		header += cellGap + p.Avatar
	}
	rows := []string{header}
	for day := 1; day <= b.Shown; day++ {
		row := code(b.Month.AddDate(0, 0, day-1).Format("02.01"))
		for _, p := range b.Participants {
			row += cellGap + cells[p.Statuses[day-1]]
		}
		rows = append(rows, row)
	}
	return rows
}

func code(s string) string {
	return "<code>" + s + "</code>"
}

func updatedAt(now time.Time) string {
	return fmt.Sprintf("обновлено %s в %s", now.Format("02.01"), now.Format("15:04"))
}
