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

package backup

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apache/skywalking-banyandb/pkg/fs/remote/s3"
)

var p = path.Join(defaultBucket, basePath)

func TestUploadAndDownload(t *testing.T) {
	fs, err := s3.NewFS(p)
	if err != nil {
		t.Fatalf("failed to create AWS S3 FS: %v", err)
	}
	defer fs.Close()

	// Define remote and local paths
	remoteFilePath := "test.txt"
	localFilePath := filepath.Join(t.TempDir(), "test.txt")
	content := "hello"

	// Create a local file
	err = os.WriteFile(localFilePath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}

	// Upload the file to S3
	file, err := os.Open(localFilePath)
	if err != nil {
		t.Fatalf("failed to open local file: %v", err)
	}
	defer file.Close()

	err = fs.Upload(context.Background(), remoteFilePath, file)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Download the file from S3
	downloadedFile, err := fs.Download(context.Background(), remoteFilePath)
	if err != nil {
		t.Fatalf("failed to download file: %v", err)
	}
	defer downloadedFile.Close()

	downloadedContent, err := io.ReadAll(downloadedFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(downloadedContent) != content {
		t.Fatalf("expected content %q, got %q", content, string(downloadedContent))
	}
}

func TestList(t *testing.T) {
	fs, err := s3.NewFS(p)
	if err != nil {
		t.Fatalf("failed to create AWS S3 FS: %v", err)
	}
	defer fs.Close()

	// List files in the S3 bucket
	_, err = fs.List(context.Background(), "")
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
}

func TestDelete(t *testing.T) {
	fs, err := s3.NewFS(p)
	if err != nil {
		t.Fatalf("failed to create AWS S3 FS: %v", err)
	}
	defer fs.Close()

	// Define remote path
	remoteFilePath := "test.txt"

	// Upload a file to S3
	err = fs.Upload(context.Background(), remoteFilePath, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// Delete the file from S3
	err = fs.Delete(context.Background(), remoteFilePath)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Verify the file was deleted
	_, err = fs.Download(context.Background(), remoteFilePath)
	if err == nil {
		t.Fatalf("expected error when downloading deleted file, got none")
	}
}
