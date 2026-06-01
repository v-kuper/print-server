package app

import (
	"context"
	"sort"
	"time"

	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/receipt"
)

func (s *ReceiptService) resolveGoogleSummary(ctx context.Context, includeMail bool, includeCalendar bool) (googleintegration.Summary, string) {
	if s.googleProvider == nil || (!includeMail && !includeCalendar) {
		return googleintegration.Summary{}, ""
	}
	var (
		summary googleintegration.Summary
		err     error
	)
	if provider, ok := s.googleProvider.(SelectiveGoogleProvider); ok {
		summary, err = provider.CurrentSelected(ctx, includeMail, includeCalendar)
	} else {
		summary, err = s.googleProvider.Current(ctx)
	}
	if err != nil {
		return googleintegration.Summary{}, "Google недоступен: " + err.Error()
	}
	return summary, ""
}

func buildCalendarSections(events []googleintegration.CalendarEvent, now time.Time) []receipt.CalendarSection {
	if len(events) == 0 {
		return nil
	}
	location := minskLocation()
	now = now.In(location)
	today := dayStart(now, location)
	tomorrow := today.AddDate(0, 0, 1)

	if calendarTomorrowMode(now, today) {
		return nonEmptyCalendarSections([]receipt.CalendarSection{
			{
				Title:  "Остаток сегодня",
				Date:   today,
				Events: calendarEventsForDay(events, today, now, true, true),
			},
			{
				Title:  "Завтра",
				Date:   tomorrow,
				Events: calendarEventsForDay(events, tomorrow, now, false, false),
			},
		})
	}

	return nonEmptyCalendarSections([]receipt.CalendarSection{{
		Title:  "Сегодня",
		Date:   today,
		Events: calendarEventsForDay(events, today, now, false, true),
	}})
}

func nonEmptyCalendarSections(sections []receipt.CalendarSection) []receipt.CalendarSection {
	result := make([]receipt.CalendarSection, 0, len(sections))
	for _, section := range sections {
		if len(section.Events) == 0 {
			continue
		}
		result = append(result, section)
	}
	return result
}

func calendarEventsForDay(events []googleintegration.CalendarEvent, day time.Time, now time.Time, onlyRemaining bool, includeUnknownDay bool) []googleintegration.CalendarEvent {
	result := make([]googleintegration.CalendarEvent, 0, len(events))
	for _, event := range events {
		if !calendarEventBelongsToDay(event, day, includeUnknownDay) {
			continue
		}
		if onlyRemaining && !calendarEventRemaining(event, now) {
			continue
		}
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := calendarEventSortTime(result[i], day.Location())
		right := calendarEventSortTime(result[j], day.Location())
		if left.Equal(right) {
			return result[i].Title < result[j].Title
		}
		return left.Before(right)
	})
	return result
}

func calendarEventBelongsToDay(event googleintegration.CalendarEvent, day time.Time, includeUnknownDay bool) bool {
	if event.Start.IsZero() {
		return includeUnknownDay
	}
	start := event.Start.In(day.Location())
	end := calendarEventEnd(event, day.Location())
	if end.IsZero() {
		end = start
	}
	if !end.After(start) {
		end = start.Add(time.Nanosecond)
	}
	return start.Before(day.AddDate(0, 0, 1)) && end.After(day)
}

func calendarEventRemaining(event googleintegration.CalendarEvent, now time.Time) bool {
	if event.Start.IsZero() {
		return true
	}
	end := calendarEventEnd(event, now.Location())
	if end.IsZero() {
		return !event.Start.In(now.Location()).Before(now)
	}
	return end.After(now)
}

func calendarEventSortTime(event googleintegration.CalendarEvent, location *time.Location) time.Time {
	if event.Start.IsZero() {
		return time.Time{}
	}
	return event.Start.In(location)
}

func calendarEventEnd(event googleintegration.CalendarEvent, location *time.Location) time.Time {
	if !event.End.IsZero() {
		return event.End.In(location)
	}
	if event.Start.IsZero() {
		return time.Time{}
	}
	start := event.Start.In(location)
	if event.AllDay {
		return dayStart(start, location).AddDate(0, 0, 1)
	}
	return start
}

func calendarContextFromSections(now time.Time, sections []receipt.CalendarSection) motivation.CalendarContext {
	context := motivation.CalendarContext{
		GeneratedAt: now.In(minskLocation()),
		Mode:        "morning",
		Sections:    make([]motivation.CalendarSectionContext, 0, len(sections)),
	}
	for _, section := range sections {
		if section.Title == "Остаток сегодня" || section.Title == "Завтра" {
			context.Mode = "evening"
		}
		sectionContext := motivation.CalendarSectionContext{
			Title:  section.Title,
			Date:   section.Date,
			Events: make([]motivation.CalendarEventContext, 0, len(section.Events)),
		}
		for _, event := range section.Events {
			sectionContext.Events = append(sectionContext.Events, motivation.CalendarEventContext{
				TimeLabel: event.TimeLabel,
				Title:     event.Title,
				Start:     event.Start,
				End:       event.End,
				AllDay:    event.AllDay,
			})
		}
		context.Sections = append(context.Sections, sectionContext)
	}
	return context
}

func minskLocation() *time.Location {
	location, err := time.LoadLocation(motivation.DefaultTimezone)
	if err != nil {
		return time.Local
	}
	return location
}

func dayStart(value time.Time, location *time.Location) time.Time {
	if location == nil {
		location = value.Location()
	}
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func calendarTomorrowMode(now time.Time, today time.Time) bool {
	return !now.Before(today.Add(15 * time.Hour))
}
