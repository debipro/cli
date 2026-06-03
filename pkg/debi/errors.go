package debi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// APIError represents a non-2xx response from the Debi API.
type APIError struct {
	StatusCode int
	RequestID  string
	Message    string
	Fields     map[string][]string
	Raw        []byte
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "request failed (HTTP %d)", e.StatusCode)
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	if len(e.Fields) > 0 {
		keys := make([]string, 0, len(e.Fields))
		for k := range e.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "\n  - %s: %s", k, strings.Join(e.Fields[k], "; "))
		}
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, "\n(request id: %s)", e.RequestID)
	}
	return b.String()
}

// parseAPIError builds an APIError from a response body.
func parseAPIError(statusCode int, requestID string, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode, RequestID: requestID, Raw: body}

	var payload struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Message = payload.Message
		apiErr.Fields = payload.Errors
	}
	return apiErr
}
