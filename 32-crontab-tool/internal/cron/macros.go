package cron

import (
	"fmt"
	"strings"
)

var macros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

// expandMacro returns the five-field form of an @macro such as @daily.
func expandMacro(tok string) (string, error) {
	lower := strings.ToLower(tok)
	if lower == "@reboot" {
		return "", fmt.Errorf("@reboot is not supported (no reboot hook in this version)")
	}
	if exp, ok := macros[lower]; ok {
		return exp, nil
	}
	return "", fmt.Errorf("unknown macro %q", tok)
}
