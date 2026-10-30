package ingest

import (
	"net/http"
	"strings"
)

// AuthCredentials encapsulates Sentry authorization parameters.
type AuthCredentials struct {
	PublicKey string
	SecretKey string
	Version   string
	Client    string
}

// ParseSentryAuth extracts authentication information from request headers and URL parameters.
func ParseSentryAuth(r *http.Request) *AuthCredentials {
	creds := &AuthCredentials{}

	// 1. Check X-Sentry-Auth header
	authHeader := r.Header.Get("X-Sentry-Auth")
	if authHeader != "" {
		parseAuthHeader(authHeader, creds)
	}

	// 2. Check Authorization header
	if creds.PublicKey == "" {
		bearer := r.Header.Get("Authorization")
		if strings.HasPrefix(bearer, "Bearer ") {
			creds.PublicKey = strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer "))
		} else if strings.HasPrefix(bearer, "Sentry ") {
			parseAuthHeader(bearer, creds)
		}
	}

	// 3. Check HTTP Basic Auth
	if creds.PublicKey == "" {
		if user, _, ok := r.BasicAuth(); ok && user != "" {
			creds.PublicKey = user
		}
	}

	// 4. Fallback to query string parameters (standard Sentry browser fallback)
	query := r.URL.Query()
	if creds.PublicKey == "" {
		creds.PublicKey = query.Get("sentry_key")
	}
	if creds.Version == "" {
		creds.Version = query.Get("sentry_version")
	}
	if creds.Client == "" {
		creds.Client = query.Get("sentry_client")
	}
	if creds.SecretKey == "" {
		creds.SecretKey = query.Get("sentry_secret")
	}

	return creds
}

func parseAuthHeader(header string, creds *AuthCredentials) {
	// Format: Sentry sentry_version=7, sentry_client=..., sentry_key=..., sentry_secret=...
	parts := strings.Split(header, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "Sentry ") {
			trimmed = strings.TrimPrefix(trimmed, "Sentry ")
		}

		kv := strings.SplitN(trimmed, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"'`)

		switch key {
		case "sentry_key":
			creds.PublicKey = val
		case "sentry_secret":
			creds.SecretKey = val
		case "sentry_version":
			creds.Version = val
		case "sentry_client":
			creds.Client = val
		}
	}
}
