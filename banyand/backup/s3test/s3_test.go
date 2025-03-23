// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package s3test

import (
	"context"
	"github.com/apache/skywalking-banyandb/pkg/fs/remote"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/apache/skywalking-banyandb/pkg/fs/remote/s3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	defaultBucket = "bydb233"
	basePath      = "basepath"
	p             = path.Join(defaultBucket, basePath)
)

var _ = Describe("S3 File System", func() {
	var fs remote.FS

	BeforeEach(func() {
		var err error
		fs, err = s3.NewFS(p)
		Expect(err).NotTo(HaveOccurred(), "failed to create AWS S3 FS")
	})

	AfterEach(func() {
		fs.Close()
	})

	Context("Upload and Download", func() {
		It("should upload and download a file successfully", func() {
			remoteFilePath := "test.txt"
			localFilePath := filepath.Join(GinkgoT().TempDir(), "test.txt")
			content := "hello"

			// Create a local file
			err := os.WriteFile(localFilePath, []byte(content), 0o600)
			Expect(err).NotTo(HaveOccurred(), "failed to write local file")

			// Upload the file
			file, err := os.Open(localFilePath)
			Expect(err).NotTo(HaveOccurred(), "failed to open local file")
			defer file.Close()

			err = fs.Upload(context.Background(), remoteFilePath, file)
			Expect(err).NotTo(HaveOccurred(), "failed to upload file")

			// Download the file
			downloadedFile, err := fs.Download(context.Background(), remoteFilePath)
			Expect(err).NotTo(HaveOccurred(), "failed to download file")
			defer downloadedFile.Close()

			downloadedContent, err := io.ReadAll(downloadedFile)
			GinkgoWriter.Println("Downloaded Content:", string(downloadedContent))

			Expect(err).NotTo(HaveOccurred(), "failed to read downloaded file")

			Expect(string(downloadedContent)).To(Equal(content), "downloaded content mismatch")
		})
	})

	Context("List", func() {
		It("should list files in the S3 bucket", func() {
			_, err := fs.List(context.Background(), "")
			Expect(err).NotTo(HaveOccurred(), "failed to list files")
		})
	})

	Context("Delete", func() {
		It("should delete a file from S3", func() {
			remoteFilePath := "test.txt"

			// Upload a file
			err := fs.Upload(context.Background(), remoteFilePath, strings.NewReader("hello"))
			Expect(err).NotTo(HaveOccurred(), "failed to upload file")

			// Delete the file
			err = fs.Delete(context.Background(), remoteFilePath)
			Expect(err).NotTo(HaveOccurred(), "failed to delete file")

			// Verify the file was deleted
			_, err = fs.Download(context.Background(), remoteFilePath)
			Expect(err).To(HaveOccurred(), "expected error when downloading deleted file, got none")
		})
	})
})
