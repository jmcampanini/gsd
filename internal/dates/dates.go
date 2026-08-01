package dates

import (
	"errors"
	"math"
	"strconv"
	"time"
)

func Parse(value string, reference time.Time) (string, error) {
	if date, ok := parseCanonical(value); ok {
		return format(date), nil
	}

	switch value {
	case "today":
		return addDays(reference, 0)
	case "tomorrow":
		return addDays(reference, 1)
	}

	if weekday, ok := parseWeekday(value); ok {
		days := (int(weekday) - int(reference.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}

		return addDays(reference, days)
	}

	if len(value) >= 3 && value[0] == '+' {
		unit := value[len(value)-1]
		if unit != 'd' && unit != 'w' {
			return "", invalidDate()
		}

		amount, ok := parseAmount(value[1 : len(value)-1])
		if !ok {
			return "", invalidDate()
		}
		if unit == 'w' {
			if amount > math.MaxInt/7 {
				return "", invalidDate()
			}
			amount *= 7
		}

		return addDays(reference, amount)
	}

	return "", invalidDate()
}

func parseCanonical(value string) (time.Time, bool) {
	date, err := time.Parse(time.DateOnly, value)
	return date, err == nil
}

func parseWeekday(value string) (time.Weekday, bool) {
	switch value {
	case "sun":
		return time.Sunday, true
	case "mon":
		return time.Monday, true
	case "tue":
		return time.Tuesday, true
	case "wed":
		return time.Wednesday, true
	case "thu":
		return time.Thursday, true
	case "fri":
		return time.Friday, true
	case "sat":
		return time.Saturday, true
	default:
		return 0, false
	}
}

func parseAmount(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, character := range []byte(value) {
		if character < '0' || character > '9' {
			return 0, false
		}
	}

	amount, err := strconv.Atoi(value)
	return amount, err == nil
}

func addDays(reference time.Time, days int) (string, error) {
	year, month, day := reference.Date()
	if !canonicalYear(year) {
		return "", invalidDate()
	}

	start := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	remaining := dayNumber(9999, time.December, 31) - dayNumber(year, month, day)
	if days > remaining {
		return "", invalidDate()
	}

	return format(start.AddDate(0, 0, days)), nil
}

func dayNumber(year int, month time.Month, day int) int {
	leapYears := (year+3)/4 - (year+99)/100 + (year+399)/400
	return year*365 + leapYears + time.Date(year, month, day, 0, 0, 0, 0, time.UTC).YearDay() - 1
}

func canonicalYear(year int) bool {
	return year >= 0 && year <= 9999
}

func format(date time.Time) string {
	return date.Format(time.DateOnly)
}

func invalidDate() error {
	return errors.New("invalid date")
}
