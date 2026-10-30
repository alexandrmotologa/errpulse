package ingest

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
)

// DecompressBody transparently decompresses HTTP payload bytes if compressed.
func DecompressBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("empty request body")
	}

	encoding := r.Header.Get("Content-Encoding")

	// Detect by magic bytes if header is not explicit
	if encoding == "" {
		if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
			encoding = "gzip"
		} else if len(raw) >= 4 && raw[0] == 0x28 && raw[1] == 0xb5 && raw[2] == 0x2f && raw[3] == 0xfd {
			encoding = "zstd"
		}
	}

	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return raw, nil // Fallback to raw if gzip header is invalid
		}
		defer gr.Close()
		return io.ReadAll(gr)

	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err == nil {
			defer zr.Close()
			return io.ReadAll(zr)
		}
		// Try raw flate
		fr := flate.NewReader(bytes.NewReader(raw))
		defer fr.Close()
		return io.ReadAll(fr)

	case "zstd":
		zr, err := zstd.NewReader(bytes.NewReader(raw))
		if err != nil {
			return raw, nil
		}
		defer zr.Close()
		return io.ReadAll(zr)

	default:
		return raw, nil
	}
}

// ParseEnvelope reads a Sentry multi-part envelope from raw bytes.
func ParseEnvelope(data []byte) (*SentryEnvelope, error) {
	reader := bufio.NewReader(bytes.NewReader(data))

	// 1. Read Envelope Header (first newline-terminated JSON)
	headerBytes, err := reader.ReadBytes('\n')
	if err != nil && len(headerBytes) == 0 {
		return nil, fmt.Errorf("failed reading envelope header: %w", err)
	}

	var envHeader SentryEnvelopeHeader
	if err := json.Unmarshal(bytes.TrimSpace(headerBytes), &envHeader); err != nil {
		return nil, fmt.Errorf("invalid envelope header JSON: %w", err)
	}

	if envHeader.EventID == "" {
		envHeader.EventID = stringsTrimHyphens(uuid.New().String())
	}

	envelope := &SentryEnvelope{
		Header: envHeader,
	}

	// 2. Read consecutive items
	for {
		itemHeaderBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && len(itemHeaderBytes) == 0 {
				break
			}
		}
		itemHeaderTrimmed := bytes.TrimSpace(itemHeaderBytes)
		if len(itemHeaderTrimmed) == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}

		var itemHeader SentryItemHeader
		if err := json.Unmarshal(itemHeaderTrimmed, &itemHeader); err != nil {
			// Some envelopes might end with blank lines
			break
		}

		var payload []byte
		if itemHeader.Length > 0 {
			payload = make([]byte, itemHeader.Length)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				return nil, fmt.Errorf("failed reading item payload of length %d: %w", itemHeader.Length, err)
			}
			// Skip trailing newline if present
			b, err := reader.ReadByte()
			if err == nil && b != '\n' {
				_ = reader.UnreadByte()
			}
		} else {
			// Read until next newline
			payload, err = reader.ReadBytes('\n')
			payload = bytes.TrimSpace(payload)
		}

		envelope.Items = append(envelope.Items, SentryEnvelopeItem{
			Header:  itemHeader,
			Payload: payload,
		})
	}

	return envelope, nil
}

func stringsTrimHyphens(s string) string {
	res := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			res = append(res, s[i])
		}
	}
	return string(res)
}
