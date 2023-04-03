package utils

import (
	"errors"
	"net/http"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
)

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var responseError *client.ResponseError
	if errors.As(err, &responseError) {
		if responseError.Response.StatusCode == http.StatusNotFound {
			return true
		}
	}
	return false
}

// IsUnauthorizedOrForbiddenError returns true for Unauthorized or Forbidden http errors
// otherwise it returns false
// NOTE: This helper only works for errors returned by go-anxcloud.
func IsUnauthorizedOrForbiddenError(err error) bool {
	if err == nil {
		return false
	}

	var (
		genericAPIClientError api.HTTPError
		legacyAPIClientError  *client.ResponseError
		statusCode            int
	)

	if errors.As(err, &genericAPIClientError) {
		statusCode = genericAPIClientError.StatusCode()
	} else if errors.As(err, &legacyAPIClientError) {
		statusCode = legacyAPIClientError.Response.StatusCode
	}

	return statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden
}

// ErrUnauthorizedForbiddenBackoff is returned if an operation is blocked due to a recent unauthorized or forbidden request
var ErrUnauthorizedForbiddenBackoff = errors.New("operation currently blocked due to client side unauthorized/forbidden request limiting")
