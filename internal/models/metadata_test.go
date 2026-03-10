package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagedEventIDForSource(t *testing.T) {
	sourceID := "source-hash"
	eventID := ManagedEventID(sourceID, "12345")

	assert.True(t, IsManagedEventIDForSource(eventID, sourceID))
	assert.False(t, IsManagedEventIDForSource(eventID, "other-source"))
}
