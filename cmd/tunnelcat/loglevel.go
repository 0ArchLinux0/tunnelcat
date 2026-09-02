// loglevel.go — shared `--log-level` flag handling.
//
// The data plane (tailcat.Server / tailcat.Client) accepts a
// logger.Logf function. We map a string level to a function
// that filters accordingly. The default is `warn` (quiet
// enough for the friend test, loud enough to surface real
// problems).
//
// This file is in the main package so it can be `import`ed
// from up.go and dial.go without a third package.
package main

import (
	"log"

	"tailscale.com/types/logger"
)

// LogLevelFromString maps "info|warn|error|silent" to a
// logger.Logf. Returns a sensible default on unknown input.
func LogLevelFromString(level string) logger.Logf {
	switch level {
	case "info":
		return log.Printf
	case "warn", "warning":
		return filterWarn
	case "error":
		return filterError
	case "silent", "off", "none":
		return logger.Discard
	default:
		// Unknown level: default to warn.
		return filterWarn
	}
}

// filterWarn passes only warnings+errors to log.Printf.
// We detect "warn" and "error" by the log format string
// tailcat uses. This is fragile but the alternative is
// changing the Logf interface; not worth it for M1.
func filterWarn(format string, args ...any) {
	// tailcat's log lines are untyped, so we just print
	// everything that doesn't look like a chatty info line.
	// For M1 we just pass everything; the friend test is
	// fine with verbose.
	log.Printf(format, args...)
}

func filterError(format string, args ...any) {
	// tailcat doesn't tag error vs warn at the Logf level.
	// Print everything for M1. A real implementation would
	// pass through to a filter.
	log.Printf(format, args...)
}
