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

// PanicIfUnauthorized panics if the provided error is due to `http.StatusUnauthorized`.
// Use this helper to slow down Engine requests by triggering a CrashLoopBackoff.
// NOTE: This helper only works for errors returned by go-anxcloud.
func PanicIfUnauthorized(err error) {
	if err == nil {
		return
	}

	var (
		genericAPIClientError api.HTTPError
		legacyAPIClientError  *client.ResponseError
	)

	if (errors.As(err, &genericAPIClientError) && genericAPIClientError.StatusCode() == http.StatusUnauthorized) ||
		(errors.As(err, &legacyAPIClientError) && legacyAPIClientError.Response.StatusCode == http.StatusUnauthorized) {
		panic("Engine responded with http.StatusUnauthorized. Please check the configured API token.")
	}
}
