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

var _ = Describe("PanicIfUnauthorized", func() {
	DescribeTable("Engine Errors", func(err error, expected types.GomegaMatcher) {
		Ω(func() { PanicIfUnauthorized(err) }).Should(expected)
	},
		Entry("legacy API unauthorized error should panic", &client.ResponseError{Response: &http.Response{StatusCode: http.StatusUnauthorized}}, PanicWith(MatchRegexp("Engine responded with http.StatusUnauthorized."))),
		Entry("legacy API other error should not panic", &client.ResponseError{Response: &http.Response{StatusCode: http.StatusNotFound}}, Not(Panic())),
		Entry("generic API unauthorized error should panic", api.NewHTTPError(http.StatusUnauthorized, "FOO", nil, nil), PanicWith(MatchRegexp("Engine responded with http.StatusUnauthorized."))),
		Entry("generic API other error should not panic", api.NewHTTPError(http.StatusNotFound, "FOO", nil, nil), Not(Panic())),
	)
})
