package usage

import (
	"time"

	"github.com/zahT3/hotseat/internal/account"
)

// A limit with no known reset time blocks an account for one 5-hour window.
const unknownLimitHold = 5 * time.Hour

// Recommend picks the account to take over from exclude: the one with the
// most quota left, skipping accounts that hit a limit that has not reset.
func Recommend(reg account.Registry, records map[string]Record, exclude string, now time.Time) (string, bool) {
	best, bestScore := "", 0.0
	for _, a := range reg.Accounts {
		if a.Name == exclude {
			continue
		}
		r := records[a.Name]
		if r.Limit.Active(now) {
			continue
		}
		five, _, _ := Effective(r.FiveHour, now)
		seven, _, _ := Effective(r.SevenDay, now)
		score := max(five, seven)
		if best == "" || score < bestScore {
			best, bestScore = a.Name, score
		}
	}
	return best, best != ""
}

// Active reports whether the account is still blocked by this limit.
func (l *LimitHit) Active(now time.Time) bool {
	if l == nil {
		return false
	}
	if l.ResetsAt > 0 {
		return l.ResetsAt > now.Unix()
	}
	return now.Sub(time.Unix(l.ObservedAt, 0)) < unknownLimitHold
}
