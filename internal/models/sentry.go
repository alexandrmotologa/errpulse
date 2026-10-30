package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// SentryEnvelopeHeader represents the leading metadata line of an envelope.
type SentryEnvelopeHeader struct {
	EventID string `json:"event_id"`
	SentAt  string `json:"sent_at"`
	DSN     string `json:"dsn"`
}

// SentryItemHeader represents metadata for an individual item inside an envelope.
type SentryItemHeader struct {
	Type        string `json:"type"` // "event", "session", "client_report", "attachment"
	Length      int    `json:"length"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename,omitempty"`
}

// SentryEnvelope contains the parsed envelope header and decoded items.
type SentryEnvelope struct {
	Header SentryEnvelopeHeader
	Items  []SentryEnvelopeItem
}

// SentryEnvelopeItem represents an item payload within the envelope.
type SentryEnvelopeItem struct {
	Header  SentryItemHeader
	Payload []byte
}

// ExtractEvent finds the first item of type 'event' within an envelope.
func (env *SentryEnvelope) ExtractEvent() (*SentryEvent, error) {
	var lastErr error
	for _, item := range env.Items {
		if item.Header.Type == "event" || item.Header.Type == "" {
			var event SentryEvent
			trimmed := bytes.TrimSpace(item.Payload)
			if err := json.Unmarshal(trimmed, &event); err != nil {
				lastErr = err
				continue
			}
			if event.EventID == "" {
				event.EventID = env.Header.EventID
			}
			return &event, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed unmarshaling event item: %w", lastErr)
	}
	return nil, fmt.Errorf("no event item found in envelope")
}

// SentryEvent represents a full crash event payload conforming to Sentry protocol.
type SentryEvent struct {
	EventID     string                   `json:"event_id"`
	Timestamp   FlexibleTime             `json:"timestamp"`
	Message     string                   `json:"message,omitempty"`
	Level       string                   `json:"level,omitempty"`
	Platform    string                   `json:"platform,omitempty"`
	Logger      string                   `json:"logger,omitempty"`
	Release     string                   `json:"release,omitempty"`
	Environment string                   `json:"environment,omitempty"`
	ServerName  string                   `json:"server_name,omitempty"`
	Transaction string                   `json:"transaction,omitempty"`
	Tags        map[string]string        `json:"tags,omitempty"`
	Extra       map[string]interface{}   `json:"extra,omitempty"`
	User        *SentryUser              `json:"user,omitempty"`
	Exception   *SentryExceptionPayload  `json:"exception,omitempty"`
	Breadcrumbs *SentryBreadcrumbPayload `json:"breadcrumbs,omitempty"`
	Request     *SentryRequest           `json:"request,omitempty"`
	SDK         *SentrySDK               `json:"sdk,omitempty"`
	Fingerprint []string                 `json:"fingerprint,omitempty"`
}

// SentryUser holds client user telemetry.
type SentryUser struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email,omitempty"`
	Username  string `json:"username,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
}

// SentryExceptionPayload wraps error exception structures.
type SentryExceptionPayload struct {
	Values []SentryException `json:"values"`
}

// SentryException describes a caught or uncaught exception.
type SentryException struct {
	Type       string            `json:"type"`
	Value      string            `json:"value"`
	Module     string            `json:"module,omitempty"`
	ThreadID   interface{}       `json:"thread_id,omitempty"`
	Stacktrace *SentryStacktrace `json:"stacktrace,omitempty"`
}

// SentryStacktrace contains chronological execution frames.
type SentryStacktrace struct {
	Frames []SentryFrame `json:"frames"`
}

// SentryFrame represents an individual stack frame.
type SentryFrame struct {
	Filename    string                 `json:"filename"`
	Function    string                 `json:"function,omitempty"`
	Module      string                 `json:"module,omitempty"`
	Lineno      int                    `json:"lineno"`
	Colno       int                    `json:"colno"`
	AbsPath     string                 `json:"abs_path,omitempty"`
	ContextLine string                 `json:"context_line,omitempty"`
	PreContext  []string               `json:"pre_context,omitempty"`
	PostContext []string               `json:"post_context,omitempty"`
	InApp       *bool                  `json:"in_app,omitempty"`
	Vars        map[string]interface{} `json:"vars,omitempty"`
}

// SentryBreadcrumbPayload handles either an array or object containing values.
type SentryBreadcrumbPayload struct {
	Values []SentryBreadcrumb `json:"values"`
}

func (b *SentryBreadcrumbPayload) UnmarshalJSON(data []byte) error {
	var list []SentryBreadcrumb
	if err := json.Unmarshal(data, &list); err == nil {
		b.Values = list
		return nil
	}

	type Alias SentryBreadcrumbPayload
	var obj Alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	b.Values = obj.Values
	return nil
}

// SentryBreadcrumb tracks user interactions and system events leading to a crash.
type SentryBreadcrumb struct {
	Type      string                 `json:"type,omitempty"`
	Category  string                 `json:"category,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Level     string                 `json:"level,omitempty"`
	Timestamp interface{}            `json:"timestamp,omitempty"`
}

// SentryRequest details the HTTP request that produced the crash.
type SentryRequest struct {
	URL         string                 `json:"url,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	QueryString string                 `json:"query_string,omitempty"`
	Cookies     string                 `json:"cookies,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Env         map[string]interface{} `json:"env,omitempty"`
}

// SentrySDK describes the reporting client library.
type SentrySDK struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// FlexibleTime accommodates float Unix timestamps or RFC3339 strings from various SDKs.
type FlexibleTime struct {
	time.Time
}

func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" || s == `""` {
		ft.Time = time.Now().UTC()
		return nil
	}

	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		t, err := time.Parse(time.RFC3339, str)
		if err == nil {
			ft.Time = t.UTC()
			return nil
		}
		t, err = time.Parse("2006-01-02T15:04:05", str)
		if err == nil {
			ft.Time = t.UTC()
			return nil
		}
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			sec := int64(f)
			nsec := int64((f - float64(sec)) * 1e9)
			ft.Time = time.Unix(sec, nsec).UTC()
			return nil
		}
	}

	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		ft.Time = time.Unix(sec, nsec).UTC()
		return nil
	}

	return fmt.Errorf("unable to parse timestamp: %s", s)
}
