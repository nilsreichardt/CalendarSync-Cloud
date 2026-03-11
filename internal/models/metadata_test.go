package models

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagedEventIDForSource(t *testing.T) {
	sourceID := "source-hash"
	eventID := ManagedEventID(sourceID, "12345")

	assert.True(t, IsManagedEventIDForSource(eventID, sourceID))
	assert.False(t, IsManagedEventIDForSource(eventID, "other-source"))
}

func TestManagedEventIDGoogleCharset(t *testing.T) {
	eventID := ManagedEventID("source-hash", "12345")
	valid := regexp.MustCompile(`^[a-v0-9]+$`)
	assert.True(t, valid.MatchString(eventID))
}
