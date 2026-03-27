package utils

import "time"

const (
	BusinessHourStart = 10
	BusinessHourEnd   = 19
)

func BeijingTime() time.Time {
	return time.Now().In(mustShanghaiLocation())
}

func IsAutoApproveTime(now time.Time) bool {
	localNow := now.In(mustShanghaiLocation())
	switch localNow.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday:
		hour := localNow.Hour()
		isBusinessHour := hour >= BusinessHourStart && hour < BusinessHourEnd
		return !isBusinessHour
	default:
		return false
	}
}

func mustShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}
