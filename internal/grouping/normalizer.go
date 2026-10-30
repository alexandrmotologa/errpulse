package grouping

import (
	"regexp"
)

var (
	reHexAddress = regexp.MustCompile(`0x[0-9a-fA-F]{4,16}`)
	reUUID       = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reISO8601    = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?`)
	rePathNumber = regexp.MustCompile(`(/[a-zA-Z_-]+/)(\d+)(/|$)`)
	reIPAddress  = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
)

// NormalizeMessage strips dynamic run-time tokens so similar errors group deterministically.
func NormalizeMessage(msg string) string {
	if msg == "" {
		return msg
	}

	res := reHexAddress.ReplaceAllString(msg, "<HEX>")
	res = reUUID.ReplaceAllString(res, "<UUID>")
	res = reISO8601.ReplaceAllString(res, "<TIME>")
	res = reIPAddress.ReplaceAllString(res, "<IP>")
	res = rePathNumber.ReplaceAllString(res, "${1}<ID>${3}")

	return res
}
