package graphapi

import "fmt"

// APIError represents a structured error returned by the Facebook Graph API.
type APIError struct {
	HTTPStatus int
	Type       string
	Message    string
	Code       int
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("graph API %d: [%s] %s (code %d)",
			e.HTTPStatus, e.Type, e.Message, e.Code)
	}
	return fmt.Sprintf("graph API %d: unknown error", e.HTTPStatus)
}

// IsRateLimit returns true if this error is a rate-limit response.
func (e *APIError) IsRateLimit() bool {
	return e.Code == 4 || e.Code == 32 || e.Code == 613
}

// IsAuthError returns true if this is an authentication/authorization error.
func (e *APIError) IsAuthError() bool {
	return e.Code == 190 || e.Type == "OAuthException"
}

// rawAPIError mirrors the JSON shape of a Graph API error body.
type rawAPIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}
