package sourcemap

import (
	"path"
	"strings"
	"sync"

	"github.com/alexandrmotologa/errpulse/internal/ingest"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// Resolver remaps minified stack traces back to original TypeScript source code.
type Resolver struct {
	db    *storage.DB
	cache map[string]*ParsedSourceMap
	mu    sync.RWMutex
}

// NewResolver creates a thread-safe source map resolver instance.
func NewResolver(db *storage.DB) *Resolver {
	return &Resolver{
		db:    db,
		cache: make(map[string]*ParsedSourceMap),
	}
}

// InvalidateCache drops parsed maps for a release when new artifacts are uploaded.
func (r *Resolver) InvalidateCache(projectID int64, release string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := releaseCacheKey(projectID, release, "")
	for k := range r.cache {
		if strings.HasPrefix(k, prefix) {
			delete(r.cache, k)
		}
	}
}

func releaseCacheKey(projectID int64, release, filename string) string {
	return string(rune(projectID)) + ":" + release + ":" + filename
}

// RemapStacktrace updates minified frames with original file, line number, and context lines.
func (r *Resolver) RemapStacktrace(projectID int64, release string, frames []ingest.SentryFrame) []ingest.SentryFrame {
	if len(frames) == 0 {
		return frames
	}

	remapped := make([]ingest.SentryFrame, len(frames))
	for i, frame := range frames {
		remapped[i] = r.remapFrame(projectID, release, frame)
	}
	return remapped
}

func (r *Resolver) remapFrame(projectID int64, release string, frame ingest.SentryFrame) ingest.SentryFrame {
	if frame.Lineno <= 0 {
		return frame
	}

	// Determine map candidates
	candidates := r.candidateMapNames(frame)
	parsedMap, sourceIdx := r.findAndParseMap(projectID, release, candidates)
	if parsedMap == nil {
		return frame
	}

	genLine := frame.Lineno - 1
	genCol := frame.Colno
	if genCol < 0 {
		genCol = 0
	}

	file, origLine, origCol, symbol, found := parsedMap.Lookup(genLine, genCol)
	if !found {
		return frame
	}

	// Update frame fields
	frame.Filename = file
	frame.Lineno = origLine + 1
	frame.Colno = origCol
	if symbol != "" {
		frame.Function = symbol
	}

	inApp := true
	frame.InApp = &inApp

	// Extract surrounding source context lines
	pre, current, post := parsedMap.GetContextLines(sourceIdx, origLine, 5)
	if current != "" {
		frame.PreContext = pre
		frame.ContextLine = current
		frame.PostContext = post
	}

	return frame
}

func (r *Resolver) candidateMapNames(frame ingest.SentryFrame) []string {
	var candidates []string
	cleanName := frame.Filename
	if idx := strings.Index(cleanName, "?"); idx != -1 {
		cleanName = cleanName[:idx]
	}
	if idx := strings.Index(cleanName, "#"); idx != -1 {
		cleanName = cleanName[:idx]
	}

	base := path.Base(cleanName)

	candidates = append(candidates,
		cleanName+".map",
		base+".map",
		"~/"+cleanName+".map",
		"~/"+base+".map",
		cleanName,
		base,
	)

	return candidates
}

func (r *Resolver) findAndParseMap(projectID int64, release string, candidates []string) (*ParsedSourceMap, int) {
	for _, cand := range candidates {
		key := releaseCacheKey(projectID, release, cand)
		r.mu.RLock()
		parsed, exists := r.cache[key]
		r.mu.RUnlock()

		if exists && parsed != nil {
			return parsed, 0
		}

		// Try loading from database
		artifact, err := r.db.GetArtifact(projectID, release, cand)
		if err == nil && artifact != nil && len(artifact.Data) > 0 {
			parsed, parseErr := ParseSourceMap(artifact.Data)
			if parseErr == nil {
				r.mu.Lock()
				r.cache[key] = parsed
				r.mu.Unlock()
				return parsed, 0
			}
		}
	}

	// Fallback: If only 1 map exists in the release artifacts, try it
	artifacts, err := r.db.ListArtifacts(projectID, release)
	if err == nil && len(artifacts) == 1 && strings.HasSuffix(artifacts[0].Filename, ".map") {
		art, getErr := r.db.GetArtifact(projectID, release, artifacts[0].Filename)
		if getErr == nil && art != nil && len(art.Data) > 0 {
			parsed, parseErr := ParseSourceMap(art.Data)
			if parseErr == nil {
				key := releaseCacheKey(projectID, release, artifacts[0].Filename)
				r.mu.Lock()
				r.cache[key] = parsed
				r.mu.Unlock()
				return parsed, 0
			}
		}
	}

	return nil, -1
}
