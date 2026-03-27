package utils

import (
	"testing"
	"time"
)

func TestIsAutoApproveTime(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
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
			if got := IsAutoApproveTime(tc.when); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
