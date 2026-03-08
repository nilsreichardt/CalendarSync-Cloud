package transformation_test

import (
	"testing"

	"github.com/inovex/CalendarSync/internal/models"
	"github.com/inovex/CalendarSync/internal/transformation"
	"github.com/stretchr/testify/assert"
)

func TestAddOriginalLink(t *testing.T) {
	tr := transformation.AddOriginalLink{}
	sink := models.Event{
		Description: "Details",
		Metadata: &models.Metadata{
			OriginalEventUri: "https://calendar.google.com/event?123",
		},
	}

	transformed, err := tr.Transform(models.Event{}, sink)
	assert.NoError(t, err)
	assert.Contains(t, transformed.Description, "Original event: https://calendar.google.com/event?123")
}
