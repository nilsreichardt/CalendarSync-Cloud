package google

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
)

func TestIsNotFound(t *testing.T) {
	t.Run("404 is treated as already missing", func(t *testing.T) {
		assert.True(t, isNotFound(&googleapi.Error{Code: http.StatusNotFound}))
	})

	t.Run("410 is treated as already missing", func(t *testing.T) {
		assert.True(t, isNotFound(&googleapi.Error{Code: http.StatusGone}))
	})

	t.Run("wrapped 410 is treated as already missing", func(t *testing.T) {
		err := fmt.Errorf("delete failed: %w", &googleapi.Error{Code: http.StatusGone})
		assert.True(t, isNotFound(err))
	})

	t.Run("other errors are not treated as already missing", func(t *testing.T) {
		assert.False(t, isNotFound(&googleapi.Error{Code: http.StatusForbidden}))
	})
}
