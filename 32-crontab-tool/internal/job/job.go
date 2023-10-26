// Package job models scheduled jobs and the crontab file that stores them.
package job

import "crontab/internal/cron"

// Job is one line of a crontab file.
type Job struct {
	Schedule *cron.Schedule
	Expr     string
	Command  string
	Name     string // from a "# name:" comment above the line, if any
}
