package filter

import (
	"github.com/inovex/CalendarSync/internal/models"
)

type AllDayEvents struct {
}

func (a AllDayEvents) Name() string {
	return "AllDayEvents"
}

func (a AllDayEvents) Filter(event models.Event) bool {
	return !event.AllDay
}

type BusyAllDayEvents struct{}

func (a BusyAllDayEvents) Name() string {
	return "BusyAllDayEvents"
}

func (a BusyAllDayEvents) Filter(event models.Event) bool {
	if !event.AllDay {
		return true
	}
	return models.NormalizeBusyStatus(event.BusyStatus) != models.BusyStatusBusy
}

type FreeAllDayEvents struct{}

func (a FreeAllDayEvents) Name() string {
	return "FreeAllDayEvents"
}

func (a FreeAllDayEvents) Filter(event models.Event) bool {
	if !event.AllDay {
		return true
	}
	return models.NormalizeBusyStatus(event.BusyStatus) != models.BusyStatusFree
}

type FreeEvents struct{}

func (a FreeEvents) Name() string {
	return "FreeEvents"
}

func (a FreeEvents) Filter(event models.Event) bool {
	return models.NormalizeBusyStatus(event.BusyStatus) != models.BusyStatusFree
}
