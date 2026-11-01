package challenge

import (
	"fmt"
	"slices"
	"strings"
)

const summaryMaxLength = 1024

type Supporter struct {
	UserID     int64
	Name       string
	Recipients int
}

type Summary struct {
	Board          Board
	Proofs         []Proof
	Supporters     []Supporter
	ChatID         int64
	StatsMessageID int64
}

type rankedParticipant struct {
	Name   string
	Prefix string
	Score  int
}

type popularProof struct {
	Name     string
	Reactors int
	URL      string
}

type summaryView struct {
	Start            string
	End              string
	Days             int
	ParticipantCount int
	Total            int
	Pending          int
	Ranking          []rankedParticipant
	Series           []string
	Streak           int
	Popular          []popularProof
	Supporters       []Supporter
	StatsURL         string
	NameLimit        int
	RankingOmitted   int
	SeriesOmitted    int
	PopularOmitted   int
	SupportOmitted   int
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
		limit := max(len(view.Ranking), len(view.Series), len(view.Popular), len(view.Supporters))
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
	view := summaryView{
		Start: b.Month.AddDate(0, 0, b.StartDay-1).Format("02.01"),
		End:   b.Month.AddDate(0, 0, b.Shown-1).Format("02.01.2006"),
		Days:  max(b.Shown-b.StartDay+1, 0), ParticipantCount: len(b.Participants),
		NameLimit: 256, StatsURL: MessageURL(s.ChatID, s.StatsMessageID),
	}
	participants := slices.Clone(b.Participants)
	slices.SortStableFunc(participants, func(a, b Participant) int { return b.Score - a.Score })
	rank := 1
	for i, p := range participants {
		if i > 0 && p.Score != participants[i-1].Score {
			rank = i + 1
		}
		prefix := fmt.Sprintf("%d.", rank)
		if p.Score > 0 && rank <= 3 {
			prefix = []string{"🥇", "🥈", "🥉"}[rank-1]
		}
		view.Ranking = append(view.Ranking, rankedParticipant{Name: p.Name, Prefix: prefix, Score: p.Score})
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
			view.Series = append(view.Series, p.Name)
		}
	}
	best := 0
	for _, p := range s.Proofs {
		if p.Day < b.StartDay || p.Day > b.Shown {
			continue
		}
		if p.Reactors < MinReactors {
			view.Pending++
		}
		if p.Reactors > best {
			best, view.Popular = p.Reactors, nil
		}
		if p.Reactors > 0 && p.Reactors == best {
			view.Popular = append(view.Popular, popularProof{Name: p.UserName, Reactors: p.Reactors, URL: MessageURL(s.ChatID, p.MessageID)})
		}
	}
	best = 0
	for _, p := range s.Supporters {
		if p.Recipients > best {
			best, view.Supporters = p.Recipients, nil
		}
		if p.Recipients > 0 && p.Recipients == best {
			if p.Name == "" {
				p.Name = fmt.Sprintf("Участник %d", p.UserID)
			}
			view.Supporters = append(view.Supporters, p)
		}
	}
	return view
}

func (v *summaryView) limit(n int) {
	v.Compact = true
	v.RankingOmitted += max(len(v.Ranking)-n, 0)
	v.Ranking = v.Ranking[:min(len(v.Ranking), n)]
	v.SeriesOmitted += max(len(v.Series)-n, 0)
	v.Series = v.Series[:min(len(v.Series), n)]
	v.PopularOmitted += max(len(v.Popular)-n, 0)
	v.Popular = v.Popular[:min(len(v.Popular), n)]
	v.SupportOmitted += max(len(v.Supporters)-n, 0)
	v.Supporters = v.Supporters[:min(len(v.Supporters), n)]
}

func MessageURL(chatID, messageID int64) string {
	chat := fmt.Sprint(chatID)
	if !strings.HasPrefix(chat, "-100") || messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://t.me/c/%s/%d", strings.TrimPrefix(chat, "-100"), messageID)
}
