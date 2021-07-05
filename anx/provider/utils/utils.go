package utils

import (
	"errors"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"net/http"
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
