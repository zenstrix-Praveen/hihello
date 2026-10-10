package payments

import "time"

func subscriptionPeriodEnd(start time.Time, period string) *time.Time {
	months := 0
	if period == "monthly" {
		months = 1
	} else if period == "yearly" {
		months = 12
	} else {
		return nil
	}
	first := time.Date(start.Year(), start.Month(), 1, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), start.Location()).AddDate(0, months, 0)
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, start.Location()).Day()
	day := start.Day()
	if day > last {
		day = last
	}
	end := time.Date(first.Year(), first.Month(), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), start.Location())
	return &end
}
