package sourcemap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SourceMapV3 represents the standard v3 JSON source map specification.
type SourceMapV3 struct {
	Version        int      `json:"version"`
	File           string   `json:"file,omitempty"`
	SourceRoot     string   `json:"sourceRoot,omitempty"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names,omitempty"`
	Mappings       string   `json:"mappings"`
}

// MappingSegment represents a resolved token mapping.
type MappingSegment struct {
	GeneratedCol int
	SourceIdx    int
	OriginalLine int
	OriginalCol  int
	NameIdx      int
}

// ParsedSourceMap caches indexed line segments for binary search lookup.
type ParsedSourceMap struct {
	SourceMap SourceMapV3
	// lines[generatedLine] = sorted list of segments
	Lines [][]MappingSegment
	// split source code lines: sourceLines[sourceIdx] = []string (lines of code)
	SourceLines [][]string
}

var b64Lookup = [256]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 62, -1, -1, -1, 63,
	52, 53, 54, 55, 56, 57, 58, 59, 60, 61, -1, -1, -1, -1, -1, -1,
	-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14,
	15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, -1, -1, -1, -1, -1,
	-1, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
	41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, -1, -1, -1, -1, -1,
}

// ParseSourceMap decodes a JSON source map and builds binary-searchable line indices.
func ParseSourceMap(data []byte) (*ParsedSourceMap, error) {
	var sm SourceMapV3
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("invalid source map json: %w", err)
	}

	lines := decodeMappings(sm.Mappings)

	// Pre-split sourcesContent lines for rapid context extraction
	sourceLines := make([][]string, len(sm.SourcesContent))
	for i, content := range sm.SourcesContent {
		if content != "" {
			normalized := strings.ReplaceAll(content, "\r\n", "\n")
			sourceLines[i] = strings.Split(normalized, "\n")
		}
	}

	return &ParsedSourceMap{
		SourceMap:   sm,
		Lines:       lines,
		SourceLines: sourceLines,
	}, nil
}

// decodeMappings converts a VLQ-encoded mappings string into structured segments.
func decodeMappings(mappings string) [][]MappingSegment {
	var lines [][]MappingSegment
	if len(mappings) == 0 {
		return lines
	}

	sourceIdx := 0
	origLine := 0
	origCol := 0
	nameIdx := 0

	// Split by semicolon (generated line boundary)
	rawLines := strings.Split(mappings, ";")
	lines = make([][]MappingSegment, len(rawLines))

	for genLineIdx, rawLine := range rawLines {
		if len(rawLine) == 0 {
			lines[genLineIdx] = nil
			continue
		}

		genCol := 0
		rawSegments := strings.Split(rawLine, ",")
		segments := make([]MappingSegment, 0, len(rawSegments))

		for _, rawSeg := range rawSegments {
			if len(rawSeg) == 0 {
				continue
			}

			pos := 0
			// Field 1: Generated Column (relative to previous segment in this line)
			deltaGenCol, nextPos, ok := decodeVLQ(rawSeg, pos)
			if !ok {
				continue
			}
			genCol += deltaGenCol
			pos = nextPos

			seg := MappingSegment{
				GeneratedCol: genCol,
				SourceIdx:    -1,
				OriginalLine: -1,
				OriginalCol:  -1,
				NameIdx:      -1,
			}

			if pos < len(rawSeg) {
				// Field 2: Source File Index (relative across entire mapping)
				deltaSource, nextPos, ok := decodeVLQ(rawSeg, pos)
				if ok {
					sourceIdx += deltaSource
					seg.SourceIdx = sourceIdx
					pos = nextPos
				}
			}

			if pos < len(rawSeg) {
				// Field 3: Original Line (relative across entire mapping)
				deltaOrigLine, nextPos, ok := decodeVLQ(rawSeg, pos)
				if ok {
					origLine += deltaOrigLine
					seg.OriginalLine = origLine
					pos = nextPos
				}
			}

			if pos < len(rawSeg) {
				// Field 4: Original Column (relative across entire mapping)
				deltaOrigCol, nextPos, ok := decodeVLQ(rawSeg, pos)
				if ok {
					origCol += deltaOrigCol
					seg.OriginalCol = origCol
					pos = nextPos
				}
			}

			if pos < len(rawSeg) {
				// Field 5: Symbol Name Index (relative across entire mapping)
				deltaName, _, ok := decodeVLQ(rawSeg, pos)
				if ok {
					nameIdx += deltaName
					seg.NameIdx = nameIdx
				}
			}

			segments = append(segments, seg)
		}

		lines[genLineIdx] = segments
	}

	return lines
}

// decodeVLQ decodes a single Variable-Length Quantity integer starting at index pos.
func decodeVLQ(s string, pos int) (val int, nextPos int, ok bool) {
	result := 0
	shift := 0
	continued := true

	for pos < len(s) && continued {
		c := s[pos]
		pos++

		v := int(b64Lookup[c])
		if v < 0 {
			return 0, pos, false
		}

		continued = (v & 32) != 0
		digit := v & 31
		result += digit << shift
		shift += 5
	}

	// First character bit 0 determines sign
	isNegative := (result & 1) == 1
	result >>= 1
	if isNegative {
		result = -result
	}

	return result, pos, true
}

// Lookup matches a 0-based (genLine, genCol) coordinate to its original source coordinate.
func (p *ParsedSourceMap) Lookup(genLine, genCol int) (file string, origLine int, origCol int, symbol string, found bool) {
	if genLine < 0 || genLine >= len(p.Lines) {
		return "", 0, 0, "", false
	}

	segments := p.Lines[genLine]
	if len(segments) == 0 {
		return "", 0, 0, "", false
	}

	// Binary search for greatest GeneratedCol <= genCol
	idx := sort.Search(len(segments), func(i int) bool {
		return segments[i].GeneratedCol > genCol
	}) - 1

	if idx < 0 {
		idx = 0
	}

	seg := segments[idx]
	if seg.SourceIdx < 0 || seg.SourceIdx >= len(p.SourceMap.Sources) {
		return "", 0, 0, "", false
	}

	file = p.SourceMap.Sources[seg.SourceIdx]
	origLine = seg.OriginalLine
	origCol = seg.OriginalCol

	if seg.NameIdx >= 0 && seg.NameIdx < len(p.SourceMap.Names) {
		symbol = p.SourceMap.Names[seg.NameIdx]
	}

	return file, origLine, origCol, symbol, true
}

// GetContextLines extracts pre-context, target line, and post-context around origLine (0-based).
func (p *ParsedSourceMap) GetContextLines(sourceIdx, origLine, radius int) (pre []string, line string, post []string) {
	if sourceIdx < 0 || sourceIdx >= len(p.SourceLines) {
		return nil, "", nil
	}

	codeLines := p.SourceLines[sourceIdx]
	if len(codeLines) == 0 || origLine < 0 || origLine >= len(codeLines) {
		return nil, "", nil
	}

	line = codeLines[origLine]

	// Pre-context
	start := origLine - radius
	if start < 0 {
		start = 0
	}
	for i := start; i < origLine; i++ {
		pre = append(pre, codeLines[i])
	}

	// Post-context
	end := origLine + 1 + radius
	if end > len(codeLines) {
		end = len(codeLines)
	}
	for i := origLine + 1; i < end; i++ {
		post = append(post, codeLines[i])
	}

	return pre, line, post
}
