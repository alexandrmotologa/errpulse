package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/alexandrmotologa/errpulse/internal/sourcemap"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// SentryCLICompatHandler provides endpoints compatible with the official sentry-cli tool.
type SentryCLICompatHandler struct {
	db       *storage.DB
	resolver *sourcemap.Resolver
}

// NewSentryCLICompatHandler creates a new sentry-cli release handler.
func NewSentryCLICompatHandler(db *storage.DB, resolver *sourcemap.Resolver) *SentryCLICompatHandler {
	return &SentryCLICompatHandler{
		db:       db,
		resolver: resolver,
	}
}

// ServeHTTP routes /api/0/... sentry-cli requests.
func (h *SentryCLICompatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path pattern: /api/0/organizations/{org}/releases/...
	pathStr := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(pathStr, "/")

	// Minimum: api/0/organizations/default/releases
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "0" || parts[2] != "organizations" || parts[4] != "releases" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 1. Create Release: POST /api/0/organizations/{org}/releases/
	if len(parts) == 5 && r.Method == http.MethodPost {
		var req struct {
			Version string `json:"version"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version": req.Version,
			"status":  "open",
		})
		return
	}

	// 2. Release Detail / Finalize: PUT/GET /api/0/organizations/{org}/releases/{version}/
	if len(parts) == 6 {
		version := parts[5]
		if r.Method == http.MethodPut || r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version": version,
				"status":  "released",
			})
			return
		}
	}

	// 3. Files endpoints: /api/0/organizations/{org}/releases/{version}/files/
	if len(parts) >= 7 && parts[6] == "files" {
		version := parts[5]

		// GET /files/ -> list files
		if r.Method == http.MethodGet {
			artifacts, err := h.db.ListArtifacts(1, version)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			var list []map[string]interface{}
			for _, a := range artifacts {
				list = append(list, map[string]interface{}{
					"id":      fmt.Sprintf("%d", a.ID),
					"name":    a.Filename,
					"headers": map[string]string{"Content-Type": a.ContentType},
					"dateCreated": a.CreatedAt,
				})
			}
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		// POST /files/ -> upload file
		if r.Method == http.MethodPost {
			err := r.ParseMultipartForm(50 << 20) // 50MB max file
			if err != nil {
				http.Error(w, "Multipart parse error", http.StatusBadRequest)
				return
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "Missing file in form", http.StatusBadRequest)
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed reading file data", http.StatusInternalServerError)
				return
			}

			filename := r.FormValue("name")
			if filename == "" {
				filename = header.Filename
			}
			if filename == "" {
				filename = path.Base(header.Filename)
			}

			contentType := header.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/json"
			}

			err = h.db.SaveArtifact(1, version, filename, contentType, data)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed storing artifact: %v", err), http.StatusInternalServerError)
				return
			}

			if h.resolver != nil {
				h.resolver.InvalidateCache(1, version)
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   fmt.Sprintf("%s-%s", version, filename),
				"name": filename,
				"size": len(data),
			})
			return
		}
	}

	http.NotFound(w, r)
}
