package bot

import (
	"strings"
	"testing"
	"time"
)

func TestDuplicateInstanceText(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	startedAt := time.Date(2026, 10, 25, 21, 30, 5, 0, loc)
	err := &duplicateInstanceError{description: "Conflict: terminated by other getUpdates request"}

	text := duplicateInstanceText("challenger_bot", "vps-2", startedAt, err)

	for _, want := range []string{"@challenger_bot", "vps-2", "25.10.2026 21:30:05 MSK", err.description} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q does not contain %q", text, want)
		}
	}
}
