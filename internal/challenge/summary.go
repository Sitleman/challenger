package challenge

import (
	"fmt"
	"html"
	"slices"
	"strings"
)

type Supporter struct {
	UserID     int64
	Name       string
	Recipients int
}

type Summary struct {
	Board      Board
	Proofs     []Proof
	Supporters []Supporter
	ChatID     int64
}

func (s Summary) Render() string {
	b := s.Board
	start := b.Month.AddDate(0, 0, b.StartDay-1)
	end := b.Month.AddDate(0, 0, b.Shown-1)
	lines := []string{
		"🏁 <b>Челлендж завершён!</b>",
		fmt.Sprintf("%s–%s · %s · %s", start.Format("02.01"), end.Format("02.01.2006"),
			counted(max(b.Shown-b.StartDay+1, 0), "день", "дня", "дней"),
			counted(len(b.Participants), "участник", "участника", "участников")),
	}
	if len(b.Participants) == 0 {
		return strings.Join(append(lines, "", "Никто не отправил "+Hashtag), "\n")
	}
	lines = append(lines, "", "<b>Результаты</b>")
	lines = append(lines, s.ranking()...)
	total := 0
	for _, p := range b.Participants {
		total += p.Score
	}
	lines = append(lines, "", "Вместе закрыли <b>"+counted(total, "день", "дня", "дней")+"</b> 💪")
	pending := 0
	for _, p := range s.Proofs {
		if p.Day >= b.StartDay && p.Day <= b.Shown && p.Reactors < MinReactors {
			pending++
		}
	}
	if pending > 0 {
		lines = append(lines, "🟡 Без подтверждения: "+counted(pending, "пруф", "пруфа", "пруфов"))
	}
	lines = append(lines, "", "🔥 <b>Самая длинная серия</b>")
	lines = append(lines, s.longestSeries()...)
	lines = append(lines, "", "❤️ <b>Самый популярный пруф</b>")
	lines = append(lines, s.popularProofs()...)
	lines = append(lines, "", "🤝 <b>Главная поддержка</b>")
	lines = append(lines, s.mainSupport()...)
	lines = append(lines, "", "Спасибо всем, кто участвовал и поддерживал других!")
	return strings.Join(lines, "\n")
}

func (s Summary) ranking() []string {
	participants := slices.Clone(s.Board.Participants)
	slices.SortStableFunc(participants, func(a, b Participant) int { return b.Score - a.Score })
	lines := make([]string, 0, len(participants))
	rank := 1
	for i, p := range participants {
		if i > 0 && p.Score != participants[i-1].Score {
			rank = i + 1
		}
		prefix := fmt.Sprintf("%d.", rank)
		if p.Score > 0 && rank <= 3 {
			prefix = []string{"🥇", "🥈", "🥉"}[rank-1]
		}
		lines = append(lines, fmt.Sprintf("%s %s — %d/%d", prefix, summaryName(p.Name), p.Score, s.Board.Shown-s.Board.StartDay+1))
	}
	return lines
}

func (s Summary) longestSeries() []string {
	best := 0
	var lines []string
	for _, p := range s.Board.Participants {
		longest, current := 0, 0
		for _, status := range p.Statuses {
			if status == StatusDone {
				current++
				longest = max(longest, current)
			} else {
				current = 0
			}
		}
		if longest == 0 || longest < best {
			continue
		}
		if longest > best {
			best, lines = longest, nil
		}
		lines = append(lines, summaryName(p.Name)+" — "+counted(longest, "день", "дня", "дней")+" подряд")
	}
	if len(lines) == 0 {
		return []string{"Нет подтверждённых дней"}
	}
	return lines
}

func (s Summary) popularProofs() []string {
	best := 0
	var lines []string
	for _, p := range s.Proofs {
		if p.Day < s.Board.StartDay || p.Day > s.Board.Shown || p.Reactors == 0 || p.Reactors < best {
			continue
		}
		if p.Reactors > best {
			best, lines = p.Reactors, nil
		}
		line := summaryName(p.UserName) + " — " + counted(p.Reactors, "реакция", "реакции", "реакций")
		if url := MessageURL(s.ChatID, p.MessageID); url != "" {
			line += " · <a href=\"" + url + "\">посмотреть</a>"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{"На пруфах нет реакций"}
	}
	return lines
}

func MessageURL(chatID, messageID int64) string {
	chat := fmt.Sprint(chatID)
	if !strings.HasPrefix(chat, "-100") || messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://t.me/c/%s/%d", strings.TrimPrefix(chat, "-100"), messageID)
}

func (s Summary) mainSupport() []string {
	best := 0
	var lines []string
	for _, p := range s.Supporters {
		if p.Recipients == 0 || p.Recipients < best {
			continue
		}
		if p.Recipients > best {
			best, lines = p.Recipients, nil
		}
		name := p.Name
		if name == "" {
			name = "Участник"
		}
		lines = append(lines, fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a> — подтвердил(а) пруфы других участников: %d", p.UserID, summaryName(name), p.Recipients))
	}
	if len(lines) == 0 {
		return []string{"Нет реакций на чужие пруфы"}
	}
	return lines
}

func summaryName(name string) string {
	return html.EscapeString(strings.Join(strings.Fields(name), " "))
}

func counted(n int, one, few, many string) string {
	word := many
	if n%100 < 11 || n%100 > 14 {
		switch n % 10 {
		case 1:
			word = one
		case 2, 3, 4:
			word = few
		}
	}
	return fmt.Sprintf("%d %s", n, word)
}
