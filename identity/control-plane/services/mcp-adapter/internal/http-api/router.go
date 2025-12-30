package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func Router(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"service": "mcp-adapter",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	registerV0(mux, logger)
	return mux
}
