package filter

import (
	"strings"

	"github.com/inovex/CalendarSync/internal/models"
)

type Days struct {
	ExcludeDays string
}

func (d Days) Name() string {
	return "Days"
}

func (d Days) Filter(event models.Event) bool {
	if strings.TrimSpace(d.ExcludeDays) == "" {
		return true
	}

	weekday := strings.ToLower(event.StartTime.Weekday().String())
	for _, day := range strings.Split(d.ExcludeDays, ",") {
		normalized := strings.ToLower(strings.TrimSpace(day))
		if normalized == "" {
			continue
		}
		if normalized == weekday {
			return false
		}
	}

	return true
}
