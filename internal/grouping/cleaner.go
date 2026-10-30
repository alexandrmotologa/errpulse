package grouping

import (
	"strings"

	"github.com/alexandrmotologa/errpulse/internal/models"
)

var vendorPatterns = []string{
	"node_modules/",
	"site-packages/",
	"/usr/lib/",
	"/usr/local/",
	"vendor/",
	"internal/runtime/",
	"runtime/",
	"webpack/bootstrap",
	"vite/client",
	"@sentry/",
	"sentry-sdk",
	"getsentry",
	"/chunks/vendor",
	"chunk-vendors",
	"polyfills",
}

// ClassifyFrame determines whether a stack frame originates from application source code.
func ClassifyFrame(frame *models.SentryFrame) bool {
	if frame.InApp != nil {
		return *frame.InApp
	}

	pathLower := strings.ToLower(frame.Filename)
	absLower := strings.ToLower(frame.AbsPath)
	moduleLower := strings.ToLower(frame.Module)

	for _, pattern := range vendorPatterns {
		if strings.Contains(pathLower, pattern) ||
			strings.Contains(absLower, pattern) ||
			strings.Contains(moduleLower, pattern) {
			inApp := false
			frame.InApp = &inApp
			return false
		}
	}

	// Browser native code wrappers
	if strings.Contains(pathLower, "<anonymous>") || strings.Contains(frame.Function, "[native code]") {
		inApp := false
		frame.InApp = &inApp
		return false
	}

	inApp := true
	frame.InApp = &inApp
	return true
}

// FilterInAppFrames partitions frames into user application frames and library frames.
func FilterInAppFrames(frames []models.SentryFrame) (inApp []models.SentryFrame, all []models.SentryFrame) {
	for i := range frames {
		frame := &frames[i]
		if ClassifyFrame(frame) {
			inApp = append(inApp, *frame)
		}
	}
	return inApp, frames
}
