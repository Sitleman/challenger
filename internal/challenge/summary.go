package challenge

import (
	"fmt"
	"slices"
	"strings"
)

const summaryMaxLength = 1024

var challengeMonths = []string{"Январский", "Февральский", "Мартовский", "Апрельский", "Майский", "Июньский", "Июльский", "Августовский", "Сентябрьский", "Октябрьский", "Ноябрьский", "Декабрьский"}
var dateMonths = []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

type Summary struct {
	Board          Board
	Proofs         []Proof
	ChatID         int64
	StatsMessageID int64
}

type summaryPerson struct {
	UserID int64
	Name   string
}

type rankedParticipant struct {
	summaryPerson
	Prefix string
	Score  int
}

type proofLink struct {
	Date string
	URL  string
}

type popularAuthor struct {
	summaryPerson
	Proofs  []proofLink
	Omitted int
}

type summaryView struct {
	Month            string
	Period           string
	Days             int
	ParticipantCount int
	Total            int
	Possible         int
	Pending          int
	Ranking          []rankedParticipant
	Series           []summaryPerson
	Streak           int
	Popular          []popularAuthor
	Reactors         int
	StatsURL         string
	NameLimit        int
	RankingOmitted   int
	SeriesOmitted    int
	PopularOmitted   int
	Compact          bool
}

func (s Summary) Render() (string, error) {
	view := s.view()
	for {
		text, err := renderMessage("summary.html.tmpl", view)
		if err != nil {
			return "", err
		}
		if messageLength(text) <= summaryMaxLength {
			return text, nil
		}
		limit := max(len(view.Ranking), len(view.Series), len(view.Popular))
		for _, author := range view.Popular {
			limit = max(limit, len(author.Proofs))
		}
		if limit > 1 {
			view.limit(max(limit/2, 1))
		} else if view.NameLimit > 16 {
			view.NameLimit = 16
		} else {
			return "", fmt.Errorf("summary exceeds %d characters", summaryMaxLength)
		}
	}
}

func (s Summary) view() summaryView {
	b := s.Board
	period := fmt.Sprintf("%d–%d %s", b.StartDay, b.Shown, dateMonths[b.Month.Month()-1])
	if b.StartDay == b.Shown {
		period = fmt.Sprintf("%d %s", b.Shown, dateMonths[b.Month.Month()-1])
	}
	view := summaryView{
		Month: challengeMonths[b.Month.Month()-1], Period: period,
		Days: max(b.Shown-b.StartDay+1, 0), ParticipantCount: len(b.Participants),
		NameLimit: 256, StatsURL: MessageURL(s.ChatID, s.StatsMessageID),
	}
	view.Possible = view.Days * view.ParticipantCount
	names := map[string]int{}
	for _, p := range b.Participants {
		names[summaryName(p.Name)]++
	}
	people := map[int64]summaryPerson{}
	for _, p := range b.Participants {
		name := summaryName(p.Name)
		if names[name] > 1 {
			name = p.Name
		}
		if name == "" {
			name = fmt.Sprintf("Участник %d", p.UserID)
		}
		people[p.UserID] = summaryPerson{UserID: p.UserID, Name: name}
	}
	participants := slices.Clone(b.Participants)
	slices.SortStableFunc(participants, func(a, b Participant) int { return b.Score - a.Score })
	rank := 1
	for i, p := range participants {
		if i > 0 && p.Score != participants[i-1].Score {
			rank = i + 1
		}
		prefix := ""
		if p.Score > 0 && rank <= 3 {
			prefix = []string{"🥇", "🥈", "🥉"}[rank-1]
		}
		person := people[p.UserID]
		view.Ranking = append(view.Ranking, rankedParticipant{summaryPerson: person, Prefix: prefix, Score: p.Score})
		view.Total += p.Score
		longest, current := 0, 0
		for _, status := range p.Statuses {
			if status == StatusDone {
				current++
				longest = max(longest, current)
			} else {
				current = 0
			}
		}
		if longest > view.Streak {
			view.Streak, view.Series = longest, nil
		}
		if longest > 0 && longest == view.Streak {
			view.Series = append(view.Series, person)
		}
	}
	for _, p := range s.Proofs {
		if p.Day < b.StartDay || p.Day > b.Shown {
			continue
		}
		if p.Reactors < MinReactors {
			view.Pending++
		}
		view.Reactors = max(view.Reactors, p.Reactors)
	}
	byAuthor := map[int64]int{}
	proofs := slices.Clone(s.Proofs)
	slices.SortStableFunc(proofs, func(a, b Proof) int { return a.Day - b.Day })
	for _, p := range proofs {
		if p.Day < b.StartDay || p.Day > b.Shown || p.Reactors == 0 || p.Reactors != view.Reactors {
			continue
		}
		index, ok := byAuthor[p.UserID]
		if !ok {
			index = len(view.Popular)
			byAuthor[p.UserID] = index
			view.Popular = append(view.Popular, popularAuthor{summaryPerson: people[p.UserID]})
		}
		date := b.Month.AddDate(0, 0, p.Day-1).Format("02.01")
		view.Popular[index].Proofs = append(view.Popular[index].Proofs, proofLink{Date: date, URL: MessageURL(s.ChatID, p.MessageID)})
	}
	return view
}

func summaryName(name string) string {
	if i := strings.LastIndex(name, " (@"); i > 0 && strings.HasSuffix(name, ")") {
		return name[:i]
	}
	return name
}

func (v *summaryView) limit(n int) {
	v.Compact = true
	v.RankingOmitted += max(len(v.Ranking)-n, 0)
	v.Ranking = v.Ranking[:min(len(v.Ranking), n)]
	v.SeriesOmitted += max(len(v.Series)-n, 0)
	v.Series = v.Series[:min(len(v.Series), n)]
	v.PopularOmitted += max(len(v.Popular)-n, 0)
	v.Popular = v.Popular[:min(len(v.Popular), n)]
	for i := range v.Popular {
		author := &v.Popular[i]
		author.Omitted += max(len(author.Proofs)-n, 0)
		author.Proofs = author.Proofs[:min(len(author.Proofs), n)]
	}
}

func MessageURL(chatID, messageID int64) string {
	chat := fmt.Sprint(chatID)
	if !strings.HasPrefix(chat, "-100") || messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://t.me/c/%s/%d", strings.TrimPrefix(chat, "-100"), messageID)
}
