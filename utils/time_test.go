package utils

import (
	"testing"
	"time"

	"archery-auto-approve/config"
)

func TestIsAutoApproveTime(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	schedule := config.ScheduleConfig{
		Timezone:            "Asia/Shanghai",
		Workdays:            []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
		BusinessHours:       config.BusinessHoursConfig{Start: "10:00", End: "19:00"},
		WeekendsAutoApprove: true,
	}

	cases := []struct {
		name string
		when time.Time
		want bool
	}{
		{
			name: "weekday before business hour",
			when: time.Date(2026, 3, 20, 9, 0, 0, 0, loc),
			want: true,
		},
		{
			name: "weekday in business hour",
			when: time.Date(2026, 3, 20, 10, 0, 0, 0, loc),
			want: false,
		},
		{
			name: "weekday after business hour",
			when: time.Date(2026, 3, 20, 19, 0, 0, 0, loc),
			want: true,
		},
		{
			name: "weekend all day",
			when: time.Date(2026, 3, 21, 14, 0, 0, 0, loc),
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAutoApproveTime(tc.when, schedule); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsAutoApproveTimeWithConfigurableWeekendRule(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	schedule := config.ScheduleConfig{
		Timezone:            "Asia/Shanghai",
		Workdays:            []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
		BusinessHours:       config.BusinessHoursConfig{Start: "10:00", End: "19:00"},
		WeekendsAutoApprove: false,
	}

	when := time.Date(2026, 3, 21, 14, 0, 0, 0, loc)
	if got := IsAutoApproveTime(when, schedule); got {
		t.Fatalf("got %v, want false", got)
	}
}
