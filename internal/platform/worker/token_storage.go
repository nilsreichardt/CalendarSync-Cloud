package worker

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/inovex/CalendarSync/internal/auth"
	"github.com/inovex/CalendarSync/internal/config"
)

type calendarTokenBinding struct {
	connectionID uuid.UUID
	token        auth.OAuth2Object
}

type dbTokenStorage struct {
	calendars map[string]calendarTokenBinding
	onWrite   func(calendarID string, token auth.OAuth2Object) error
}

func (d *dbTokenStorage) Setup(_ config.AuthStorage, _ string) error {
	return nil
}

func (d *dbTokenStorage) WriteCalendarAuth(newCal auth.CalendarAuth) (bool, error) {
	binding, ok := d.calendars[newCal.CalendarID]
	if !ok {
		return false, fmt.Errorf("unknown calendar id %s", newCal.CalendarID)
	}
	binding.token = newCal.OAuth2
	d.calendars[newCal.CalendarID] = binding
	if d.onWrite != nil {
		if err := d.onWrite(newCal.CalendarID, newCal.OAuth2); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (d *dbTokenStorage) ReadCalendarAuth(calendarID string) (*auth.CalendarAuth, error) {
	binding, ok := d.calendars[calendarID]
	if !ok {
		return nil, nil
	}
	return &auth.CalendarAuth{
		CalendarID: calendarID,
		OAuth2:     binding.token,
	}, nil
}

func (d *dbTokenStorage) RemoveCalendarAuth(calendarID string) error {
	delete(d.calendars, calendarID)
	return nil
}
