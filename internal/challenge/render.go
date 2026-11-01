package challenge

import "time"

var monthNames = []string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

var cells = map[Status]string{
	StatusOpen:    "　",
	StatusMissed:  "❌",
	StatusPending: "🟡",
	StatusDone:    "✅",
}

type calendarRow struct {
	Date  string
	Cells []string
}

type boardView struct {
	Board
	MonthName   string
	Year        int
	Start       string
	End         string
	Days        int
	TotalDays   int
	Rows        []calendarRow
	UpdatedDate string
	UpdatedTime string
}

func (b Board) Render(now time.Time) (string, error) {
	now = now.In(b.Month.Location())
	view := boardView{
		Board: b, MonthName: monthNames[b.Month.Month()-1], Year: b.Month.Year(),
		Start:       b.Month.AddDate(0, 0, b.StartDay-1).Format("02.01"),
		End:         b.Month.AddDate(0, 0, b.Shown-1).Format("02.01"),
		Days:        max(b.Shown-b.StartDay+1, 0),
		TotalDays:   b.DaysInMonth - b.StartDay + 1,
		UpdatedDate: now.Format("02.01"), UpdatedTime: now.Format("15:04"),
	}
	for day := b.StartDay; day <= b.Shown; day++ {
		row := calendarRow{Date: b.Month.AddDate(0, 0, day-1).Format("02.01")}
		for _, p := range b.Participants {
			row.Cells = append(row.Cells, cells[p.Statuses[day-b.StartDay]])
		}
		view.Rows = append(view.Rows, row)
	}
	return renderMessage("board.html.tmpl", view)
}
