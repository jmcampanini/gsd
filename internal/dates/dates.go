package dates

import (
	"errors"
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
		year, month, day := reference.Date()
		if !canonicalYear(year) {
			return "", invalidDate()
		}

		current := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		days := (int(weekday) - int(current.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}

		return addDays(reference, uint64(days))
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
			if amount > ^uint64(0)/7 {
				return "", invalidDate()
			}
			amount *= 7
		}

		return addDays(reference, amount)
	}

	return "", invalidDate()
}

func parseCanonical(value string) (time.Time, bool) {
	if len(value) != len("2006-01-02") || value[4] != '-' || value[7] != '-' {
		return time.Time{}, false
	}
	for index, character := range []byte(value) {
		if index == 4 || index == 7 {
			continue
		}
		if character < '0' || character > '9' {
			return time.Time{}, false
		}
	}

	year := decimal(value[0:4])
	month := decimal(value[5:7])
	day := decimal(value[8:10])
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != year || int(date.Month()) != month || date.Day() != day {
		return time.Time{}, false
	}

	return date, true
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

func parseAmount(value string) (uint64, bool) {
	if value == "" {
		return 0, false
	}
	for _, character := range []byte(value) {
		if character < '0' || character > '9' {
			return 0, false
		}
	}

	amount, err := strconv.ParseUint(value, 10, 64)
	return amount, err == nil
}

func addDays(reference time.Time, days uint64) (string, error) {
	year, month, day := reference.Date()
	if !canonicalYear(year) {
		return "", invalidDate()
	}

	start := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	remaining := dayNumber(9999, time.December, 31) - dayNumber(year, month, day)
	if days > uint64(remaining) {
		return "", invalidDate()
	}

	return format(start.AddDate(0, 0, int(days))), nil
}

func dayNumber(year int, month time.Month, day int) int {
	leapYears := (year+3)/4 - (year+99)/100 + (year+399)/400
	return year*365 + leapYears + time.Date(year, month, day, 0, 0, 0, 0, time.UTC).YearDay() - 1
}

func decimal(value string) int {
	result := 0
	for _, digit := range []byte(value) {
		result = result*10 + int(digit-'0')
	}

	return result
}

func canonicalYear(year int) bool {
	return year >= 0 && year <= 9999
}

func format(date time.Time) string {
	return date.Format("2006-01-02")
}

func invalidDate() error {
	return errors.New("invalid date")
}
