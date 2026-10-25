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
	lines := []string{b.title(), b.period(), "", "👥 <b>Участники</b>"}
	if len(b.Participants) == 0 {
		lines = append(lines, "Пока никто не отправил "+Hashtag)
	}
	for _, p := range b.Participants {
		lines = append(lines, p.summary())
	}
	if len(b.Participants) > 0 {
		lines = append(lines, "")
		lines = append(lines, b.table()...)
	}
	lines = append(lines, "", legend, updatedAt(now.In(b.Month.Location())))
	return strings.Join(lines, "\n")
}

func (b Board) title() string {
	return fmt.Sprintf("📊 Челлендж · %s %d", monthNames[b.Month.Month()-1], b.Month.Year())
}

func (b Board) period() string {
	start := b.Month.AddDate(0, 0, b.StartDay-1).Format("02.01")
	return fmt.Sprintf("Старт %s · день %d из %d", start, max(b.Shown-b.StartDay+1, 0), b.DaysInMonth-b.StartDay+1)
}

func (p Participant) summary() string {
	s := fmt.Sprintf("%s %s — %d/%d", p.Avatar, html.EscapeString(p.Name), p.Score, p.Closed)
	if p.Streak > 0 {
		s += fmt.Sprintf(" · 🔥%d", p.Streak)
	}
	return s
}

func (b Board) table() []string {
	header := code("     ")
	for _, p := range b.Participants {
		header += cellGap + p.Avatar
	}
	rows := []string{header}
	for day := b.StartDay; day <= b.Shown; day++ {
		row := code(b.Month.AddDate(0, 0, day-1).Format("02.01"))
		for _, p := range b.Participants {
			row += cellGap + cells[p.Statuses[day-b.StartDay]]
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
