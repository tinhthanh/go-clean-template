package response

import "time"

// Error is the simple error response (backward compatible).
type Error struct {
	Error string `json:"error" example:"message"`
}

// ErrorResponse is the standardized API error response for production use.
type ErrorResponse struct {
	RequestID string `json:"request_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code      string `json:"code"                 example:"VALIDATION_ERROR"`
	Message   string `json:"message"              example:"invalid input"`
	Timestamp string `json:"timestamp"             example:"2026-02-22T21:58:00Z"`
}

// NewErrorResponse creates a new ErrorResponse with the current timestamp.
func NewErrorResponse(requestID, code, message string) ErrorResponse {
	return ErrorResponse{
		RequestID: requestID,
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
