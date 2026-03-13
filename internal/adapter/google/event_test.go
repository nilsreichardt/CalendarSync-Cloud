package google

import (
	"testing"

	"github.com/inovex/CalendarSync/internal/models"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/calendar/v3"
)

func Test_ensureMetadata(t *testing.T) {
	adapterSourceID := "testSourceId"
	tt := []struct {
		name             string
		event            calendar.Event
		expectedMetadata *models.Metadata
	}{
		{
			name:             "empty input event",
			event:            calendar.Event{},
			expectedMetadata: models.NewEventMetadata("", "", adapterSourceID),
		},
		{
			name: "complete input event with metadata",
			event: calendar.Event{
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{
						"EventID":          "test",
						"OriginalEventUri": "test",
						"ContentHash":      "test",
						"SourceID":         "test",
					},
				},
			},
			expectedMetadata: &models.Metadata{
				SyncID:           "test",
				OriginalEventUri: "test",
				SourceID:         "test",
				Managed:          true,
			},
		},
		{
			name: "missing event property EventID",
			event: calendar.Event{
				Id:       "eventID",
				HtmlLink: "htmlLink",
				Etag:     "eTag",
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{
						"OriginalEventUri": "test",
						"ContentHash":      "test",
						"SourceID":         "test",
					},
				},
			},
			expectedMetadata: models.NewEventMetadata("eventID", "htmlLink", adapterSourceID),
		},
		{
			name: "missing event property OriginalEventUri",
			event: calendar.Event{
				Id:       "eventID",
				HtmlLink: "htmlLink",
				Etag:     "eTag",
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{
						"SyncID":      "test",
						"ContentHash": "test",
						"SourceID":    "test",
					},
				},
			},
			expectedMetadata: models.NewEventMetadata("eventID", "htmlLink", adapterSourceID),
		},
		{
			name: "missing event property ContentHash",
			event: calendar.Event{
				Id:       "eventID",
				HtmlLink: "htmlLink",
				Etag:     "eTag",
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{
						"SyncID":           "test",
						"OriginalEventUri": "test",
						"SourceID":         "test",
					},
				},
			},
			expectedMetadata: models.NewEventMetadata("eventID", "htmlLink", adapterSourceID),
		},
		{
			name: "missing event property SourceID",
			event: calendar.Event{
				Id:       "eventID",
				HtmlLink: "htmlLink",
				Etag:     "eTag",
				ExtendedProperties: &calendar.EventExtendedProperties{
					Private: map[string]string{
						"SyncID":           "test",
						"OriginalEventUri": "test",
						"ContentHash":      "test",
					},
				},
			},
			expectedMetadata: models.NewEventMetadata("eventID", "htmlLink", adapterSourceID),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			metadata := ensureMetadata(&tc.event, adapterSourceID)

			assert.Equal(t, tc.expectedMetadata, metadata)
		})
	}
}

func Test_transparencyMapping(t *testing.T) {
	assert.Equal(t, models.BusyStatusBusy, transparencyToBusyStatus(""))
	assert.Equal(t, models.BusyStatusBusy, transparencyToBusyStatus("opaque"))
	assert.Equal(t, models.BusyStatusFree, transparencyToBusyStatus("transparent"))

	assert.Equal(t, "opaque", busyStatusToTransparency(models.BusyStatusBusy))
	assert.Equal(t, "transparent", busyStatusToTransparency(models.BusyStatusFree))
	assert.Equal(t, "opaque", busyStatusToTransparency(""))
}

func Test_calendarEventToEventBusyStatus(t *testing.T) {
	source := &calendar.Event{
		Transparency: "transparent",
		Start:        &calendar.EventDateTime{Date: "2026-03-13"},
		End:          &calendar.EventDateTime{Date: "2026-03-14"},
	}

	mapped := calendarEventToEvent(source, "source")
	assert.Equal(t, models.BusyStatusFree, mapped.BusyStatus)
}
