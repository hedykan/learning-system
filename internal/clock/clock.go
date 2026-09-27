// Package clock is the Runtime's single source of "now". The LEARN_NOW
// environment variable (RFC3339) overrides it for acceptance tests only.
package clock

import (
	"os"
	"time"
)

// Now returns LEARN_NOW when set and valid, otherwise the system time.
func Now() time.Time {
	if v := os.Getenv("LEARN_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Now()
}

// Date formats t as a local calendar day.
func Date(t time.Time) string { return t.Local().Format("2006-01-02") }
