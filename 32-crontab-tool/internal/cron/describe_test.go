package cron

import "testing"

func TestDescribe(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"0 9 * * SAT", "At 09:00, only on Saturday"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"0 0 1 * *", "At 00:00, on day 1 of the month"},
		{"0 22 * * 1-5", "At 22:00, Monday through Friday"},
		{"0 2 * * *", "At 02:00 every day"},

		// Frequency forms.
		{"* * * * *", "Every minute"},
		{"*/5 * * * *", "Every 5 minutes"},
		{"0 * * * *", "Every hour"},
		{"30 * * * *", "At minute 30 past every hour"},

		// Time of day with day/month restrictions.
		{"0 0 * * 0", "At 00:00, only on Sunday"},
		{"15 14 1 * *", "At 14:15, on day 1 of the month"},
		{"0 0 1 1 *", "At 00:00, on day 1 of the month, in January"},
		{"0 12 * 6 *", "At 12:00 every day, in June"},
		{"0 0 * 1-3 *", "At 00:00 every day, in January through March"},

		// Lists and ranges.
		{"0 0 1,15 * *", "At 00:00, on days 1 and 15 of the month"},
		{"0 9 * * 1,3,5", "At 09:00, on Monday, Wednesday and Friday"},
		{"0 0 13 * FRI", "At 00:00, on day 13 of the month or only on Friday"},

		// Sub-daily frequency with restrictions.
		{"*/15 * * * 1-5", "Every 15 minutes, Monday through Friday"},

		// Macros (expand then describe).
		{"@daily", "At 00:00 every day"},
		{"@hourly", "Every hour"},
		{"@weekly", "At 00:00, only on Sunday"},
		{"@yearly", "At 00:00, on day 1 of the month, in January"},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			s, err := Parse(tc.expr)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.expr, err)
			}
			got := Describe(s)
			if got != tc.want {
				t.Errorf("Describe(%q) = %q, want %q", tc.expr, got, tc.want)
			}
		})
	}
}
