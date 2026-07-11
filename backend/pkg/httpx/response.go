package httpx

import (
	"encoding/json"
	"net/http"
)

type Meta struct {
	RequestID string `json:"requestId"`
}

type SuccessResponse struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	WriteJSON(w, status, SuccessResponse{Data: data, Meta: Meta{RequestID: RequestID(r.Context())}})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	if details == nil {
		details = []any{}
	}
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Details: details},
		Meta:  Meta{RequestID: RequestID(r.Context())},
	})
}
