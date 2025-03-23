package s3test_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestS3test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "S3test Suite")
}
