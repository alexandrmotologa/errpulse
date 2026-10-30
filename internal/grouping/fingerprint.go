package grouping

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/alexandrmotologa/errpulse/internal/models"
)

// GenerateFingerprint computes a deterministic issue fingerprint and summary fields.
func GenerateFingerprint(event *models.SentryEvent) (fingerprint, title, culprit, exceptionType string) {
	// 1. Explicit client fingerprint override
	if len(event.Fingerprint) > 0 {
		h := sha256.New()
		for _, part := range event.Fingerprint {
			h.Write([]byte(part))
			h.Write([]byte{0})
		}
		fingerprint = hex.EncodeToString(h.Sum(nil))[:32]
		title = event.Message
		if title == "" && event.Exception != nil && len(event.Exception.Values) > 0 {
			title = event.Exception.Values[0].Type
		}
		culprit = event.Transaction
		exceptionType = "Custom"
		return fingerprint, title, culprit, exceptionType
	}

	// 2. Exception-based fingerprinting
	if event.Exception != nil && len(event.Exception.Values) > 0 {
		// In Sentry, the last exception in values is the most specific inner exception
		ex := event.Exception.Values[len(event.Exception.Values)-1]
		exceptionType = ex.Type
		if exceptionType == "" {
			exceptionType = "Error"
		}

		normalizedVal := NormalizeMessage(ex.Value)
		if normalizedVal != "" {
			title = fmt.Sprintf("%s: %s", exceptionType, ex.Value)
		} else {
			title = exceptionType
		}

		// Frame extraction
		var relevantFrames []models.SentryFrame
		if ex.Stacktrace != nil && len(ex.Stacktrace.Frames) > 0 {
			inApp, all := FilterInAppFrames(ex.Stacktrace.Frames)
			if len(inApp) > 0 {
				relevantFrames = inApp
			} else {
				relevantFrames = all
			}
		}

		h := sha256.New()
		h.Write([]byte(exceptionType))
		h.Write([]byte{0})
		h.Write([]byte(normalizedVal))
		h.Write([]byte{0})

		// Select top 3 innermost frames (last in array)
		numFrames := len(relevantFrames)
		startIndex := numFrames - 3
		if startIndex < 0 {
			startIndex = 0
		}

		for i := numFrames - 1; i >= startIndex; i-- {
			f := relevantFrames[i]
			cleanFile := path.Base(f.Filename)
			h.Write([]byte(f.Function))
			h.Write([]byte{0})
			h.Write([]byte(cleanFile))
			h.Write([]byte{0})
		}

		fingerprint = hex.EncodeToString(h.Sum(nil))[:32]

		// Determine culprit from the most specific frame
		if numFrames > 0 {
			top := relevantFrames[numFrames-1]
			baseFile := path.Base(top.Filename)
			if top.Function != "" {
				culprit = fmt.Sprintf("%s(%s:%d)", top.Function, baseFile, top.Lineno)
			} else {
				culprit = fmt.Sprintf("%s:%d", baseFile, top.Lineno)
			}
		} else if event.Transaction != "" {
			culprit = event.Transaction
		} else {
			culprit = "unknown"
		}

		return fingerprint, title, culprit, exceptionType
	}

	// 3. Fallback to message-based grouping
	exceptionType = "Message"
	rawMsg := event.Message
	if rawMsg == "" {
		rawMsg = "Unspecified runtime exception"
	}

	normalized := NormalizeMessage(rawMsg)
	title = strings.TrimSpace(rawMsg)
	if len(title) > 200 {
		title = title[:197] + "..."
	}

	h := sha256.New()
	h.Write([]byte("message"))
	h.Write([]byte{0})
	h.Write([]byte(normalized))
	fingerprint = hex.EncodeToString(h.Sum(nil))[:32]

	if event.Transaction != "" {
		culprit = event.Transaction
	} else {
		culprit = "main"
	}

	return fingerprint, title, culprit, exceptionType
}
