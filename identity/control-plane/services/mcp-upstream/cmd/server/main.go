package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

type toolCallParams struct {
	Name string `json:"name"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"service": "mcp-upstream",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var params toolCallParams
		_ = json.Unmarshal(req.Params, &params)

		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"ok":   true,
				"tool": params.Name,
			},
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	addr := ":" + getenv("PORT", "8090")
	logger.Info("starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
