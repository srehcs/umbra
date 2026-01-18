package protocol

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error      ErrorBody `json:"error"`
	RequestID  string    `json:"request_id,omitempty"`
	DecisionID string    `json:"decision_id,omitempty"`
	TraceID    string    `json:"trace_id,omitempty"`
}

// WriteErrorResponse emits the standard error envelope and headers.
func WriteErrorResponse(w http.ResponseWriter, status int, code, message, requestID, decisionID, traceID string, details []ErrorDetail) {
	if strings.TrimSpace(requestID) == "" {
		requestID = uuid.NewString()
	}
	w.Header().Set("content-type", "application/json")
	w.Header().Set("x-umbra-request-id", requestID)
	w.Header().Set("x-request-id", requestID)
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error:      ErrorBody{Code: code, Message: message, Details: details},
		RequestID:  requestID,
		DecisionID: decisionID,
		TraceID:    traceID,
	}
	if len(details) == 0 {
		resp.Error.Details = nil
	}
	_ = json.NewEncoder(w).Encode(resp)
}
