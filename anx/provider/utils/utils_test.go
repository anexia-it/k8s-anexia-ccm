package utils

import (
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utility functions")
}

var _ = DescribeTable("IsUnauthorizedOrForbiddenError", func(err error, expected types.GomegaMatcher) {
	Expect(IsUnauthorizedOrForbiddenError(err)).To(expected)
},
	Entry("legacy API unauthorized error", &client.ResponseError{Response: &http.Response{StatusCode: http.StatusUnauthorized}}, BeTrue()),
	Entry("legacy API other error", &client.ResponseError{Response: &http.Response{StatusCode: http.StatusNotFound}}, BeFalse()),
	Entry("generic API unauthorized error", api.NewHTTPError(http.StatusUnauthorized, "FOO", nil, nil), BeTrue()),
	Entry("generic API other error", api.NewHTTPError(http.StatusNotFound, "FOO", nil, nil), BeFalse()),
)
