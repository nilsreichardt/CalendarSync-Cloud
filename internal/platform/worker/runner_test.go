package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCleanupWindow(t *testing.T) {
	start, end := cleanupWindow()

	assert.Equal(t, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC), end)
}
