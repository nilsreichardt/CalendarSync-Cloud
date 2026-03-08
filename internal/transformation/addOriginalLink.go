package transformation

import (
	"fmt"
	"strings"

	"github.com/inovex/CalendarSync/internal/models"
)

type AddOriginalLink struct{}

func (a AddOriginalLink) Name() string {
	return "AddOriginalLink"
}

func (a AddOriginalLink) Transform(_ models.Event, sink models.Event) (models.Event, error) {
	if sink.Metadata == nil || sink.Metadata.OriginalEventUri == "" {
		return sink, nil
	}
	linkLine := fmt.Sprintf("Original event: %s", sink.Metadata.OriginalEventUri)
	if strings.Contains(sink.Description, linkLine) {
		return sink, nil
	}
	if sink.Description == "" {
		sink.Description = linkLine
	} else {
		sink.Description = sink.Description + "\n\n" + linkLine
	}
	return sink, nil
}
