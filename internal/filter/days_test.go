package filter_test

import (
	"testing"
	"time"

	"github.com/inovex/CalendarSync/internal/filter"
	"github.com/inovex/CalendarSync/internal/models"
)

func TestDaysFilter(t *testing.T) {
	const timeFormat = "2006-01-02T15:04"

	weekendStart, err := time.Parse(timeFormat, "2026-03-08T09:00")
	if err != nil {
		t.Fatal(err)
	}
	weekdayStart, err := time.Parse(timeFormat, "2026-03-09T09:00")
	if err != nil {
		t.Fatal(err)
	}

	sourceEvents := []models.Event{
		{
			Title:     "Sunday event",
			StartTime: weekendStart,
			EndTime:   weekendStart.Add(time.Hour),
		},
		{
			Title:     "Monday event",
			StartTime: weekdayStart,
			EndTime:   weekdayStart.Add(time.Hour),
		},
	}

	expectedSinkEvents := []models.Event{sourceEvents[1]}

	eventFilter := filter.Days{
		ExcludeDays: "Saturday,Sunday",
	}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}

func TestDaysFilterEmptyConfig(t *testing.T) {
	const timeFormat = "2006-01-02T15:04"

	start, err := time.Parse(timeFormat, "2026-03-09T09:00")
	if err != nil {
		t.Fatal(err)
	}

	sourceEvents := []models.Event{
		{
			Title:     "Monday event",
			StartTime: start,
			EndTime:   start.Add(time.Hour),
		},
	}

	expectedSinkEvents := sourceEvents

	eventFilter := filter.Days{
		ExcludeDays: "",
	}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}
