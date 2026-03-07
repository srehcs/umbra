package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func Router(logger *slog.Logger) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"service": "pdp",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// V0 endpoints are wired in service-specific files.
	if err := registerV0(mux, logger); err != nil {
		return nil, err
	}
	return mux, nil
}
