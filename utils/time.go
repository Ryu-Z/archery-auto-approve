package utils

import (
	"strings"
	"time"

	"archery-auto-approve/config"
)

func BeijingTime() time.Time {
	return time.Now().In(mustShanghaiLocation())
}

func IsAutoApproveTime(now time.Time, schedule config.ScheduleConfig) bool {
	loc := mustLocation(schedule.Timezone)
	localNow := now.In(loc)
	weekday := strings.ToLower(localNow.Weekday().String())

	if !containsWeekday(schedule.Workdays, weekday) {
		return schedule.WeekendsAutoApprove
	}

	startMinute, ok := parseClock(schedule.BusinessHours.Start)
	if !ok {
		return false
	}
	endMinute, ok := parseClock(schedule.BusinessHours.End)
	if !ok {
		return false
	}

	currentMinute := localNow.Hour()*60 + localNow.Minute()
	isBusinessHour := currentMinute >= startMinute && currentMinute < endMinute
	return !isBusinessHour
}

func containsWeekday(days []string, weekday string) bool {
	for _, day := range days {
		if strings.EqualFold(strings.TrimSpace(day), weekday) {
			return true
		}
	}
	return false
}

func parseClock(value string) (int, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func mustShanghaiLocation() *time.Location {
	return mustLocation("Asia/Shanghai")
}

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
