package filter_test

import (
	"testing"

	"github.com/inovex/CalendarSync/internal/filter"
	"github.com/inovex/CalendarSync/internal/models"
)

// All Day Events should be filtered
func TestAllDayEventsFilter(t *testing.T) {
	sourceEvents := []models.Event{
		{
			ICalUID:     "testId",
			ID:          "testUid",
			Title:       "test",
			Description: "bar",
			AllDay:      true,
		},
		{
			ICalUID:     "testId2",
			ID:          "testUid2",
			Title:       "Test 2",
			Description: "bar",
			AllDay:      false,
		},
		{
			ICalUID:     "testId3",
			ID:          "testUid2",
			Title:       "foo",
			Description: "bar",
		},
	}

	expectedSinkEvents := []models.Event{sourceEvents[1], sourceEvents[2]}

	eventFilter := filter.AllDayEvents{}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}

func TestBusyAllDayEventsFilter(t *testing.T) {
	sourceEvents := []models.Event{
		{
			Title:      "busy all-day",
			AllDay:     true,
			BusyStatus: models.BusyStatusBusy,
		},
		{
			Title:      "free all-day",
			AllDay:     true,
			BusyStatus: models.BusyStatusFree,
		},
		{
			Title:      "timed busy",
			AllDay:     false,
			BusyStatus: models.BusyStatusBusy,
		},
	}

	expectedSinkEvents := []models.Event{sourceEvents[1], sourceEvents[2]}

	eventFilter := filter.BusyAllDayEvents{}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}

func TestFreeAllDayEventsFilter(t *testing.T) {
	sourceEvents := []models.Event{
		{
			Title:      "busy all-day",
			AllDay:     true,
			BusyStatus: models.BusyStatusBusy,
		},
		{
			Title:      "free all-day",
			AllDay:     true,
			BusyStatus: models.BusyStatusFree,
		},
		{
			Title:      "timed free",
			AllDay:     false,
			BusyStatus: models.BusyStatusFree,
		},
	}

	expectedSinkEvents := []models.Event{sourceEvents[0], sourceEvents[2]}

	eventFilter := filter.FreeAllDayEvents{}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}

func TestFreeEventsFilter(t *testing.T) {
	sourceEvents := []models.Event{
		{
			Title:      "busy timed",
			AllDay:     false,
			BusyStatus: models.BusyStatusBusy,
		},
		{
			Title:      "free timed",
			AllDay:     false,
			BusyStatus: models.BusyStatusFree,
		},
		{
			Title:      "free all-day",
			AllDay:     true,
			BusyStatus: models.BusyStatusFree,
		},
	}

	expectedSinkEvents := []models.Event{sourceEvents[0]}

	eventFilter := filter.FreeEvents{}
	checkEventFilter(t, eventFilter, sourceEvents, expectedSinkEvents)
}
