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
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	commonv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/common/v1"
	"github.com/apache/skywalking-banyandb/banyand/backup/snapshot"
	"github.com/apache/skywalking-banyandb/banyand/internal/storage"
	"github.com/apache/skywalking-banyandb/pkg/fs/remote"
	"github.com/apache/skywalking-banyandb/pkg/fs/remote/local"
	"github.com/apache/skywalking-banyandb/pkg/fs/remote/s3"
)

var (
	defaultBucket = "bydb233"
	basePath      = "basepath"
)

func testRestoreDownload(t *testing.T, fsProvider func() (remote.FS, error)) {
	localRestoreDir := t.TempDir()

	fs, err := fsProvider()
	if err != nil {
		t.Fatalf("failed to create remote FS: %v", err)
	}

	timeDir := "2023-10-10"
	remoteFilePath := filepath.Join(timeDir, snapshot.CatalogName(commonv1.Catalog_CATALOG_STREAM), "test.txt")
	content := "hello"

	err = fs.Upload(context.Background(), remoteFilePath, strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	err = restoreCatalog(fs, timeDir, localRestoreDir, commonv1.Catalog_CATALOG_STREAM)
	if err != nil {
		t.Fatalf("restoreCatalog failed: %v", err)
	}

	localFilePath := filepath.Join(localRestoreDir, snapshot.CatalogName(commonv1.Catalog_CATALOG_STREAM), storage.DataDir, "test.txt")
	got, err := os.ReadFile(localFilePath)
	if err != nil {
		t.Fatalf("failed to read local file: %v", err)
	}
	if string(got) != content {
		t.Fatalf("expected content %q, got %q", content, string(got))
	}
}

func TestRestoreDownload(t *testing.T) {
	testRestoreDownload(t, func() (remote.FS, error) {
		remoteDir := t.TempDir()
		return local.NewFS(remoteDir)
	})
}

func TestRestoreDownloadS3(t *testing.T) {
	testRestoreDownload(t, func() (remote.FS, error) {
		remoteDir := path.Join(defaultBucket, t.TempDir())
		return s3.NewFS(remoteDir)
	})
}

func testRestoreDelete(t *testing.T, fsProvider func() (remote.FS, error)) {
	localRestoreDir := t.TempDir()
	fs, err := fsProvider()
	if err != nil {
		t.Fatalf("failed to create remote FS: %v", err)
	}

	streamDir := filepath.Join(localRestoreDir, "stream", storage.DataDir)
	if err = os.MkdirAll(streamDir, storage.DirPerm); err != nil {
		t.Fatalf("failed to create local stream directory: %v", err)
	}

	extraFilePath := filepath.Join(streamDir, "old.txt")
	extraContent := "stale"
	if err = os.WriteFile(extraFilePath, []byte(extraContent), 0o600); err != nil {
		t.Fatalf("failed to write extra local file: %v", err)
	}

	timeDir := "2023-10-10"
	err = restoreCatalog(fs, timeDir, localRestoreDir, commonv1.Catalog_CATALOG_STREAM)
	if err != nil {
		t.Fatalf("restoreCatalog failed: %v", err)
	}

	if _, err := os.Stat(extraFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected extra file %q to be deleted", extraFilePath)
	}
}

func TestRestoreDelete(t *testing.T) {
	testRestoreDelete(t, func() (remote.FS, error) {
		remoteDir := t.TempDir()
		return local.NewFS(remoteDir)
	})
}

func TestRestoreDeleteS3(t *testing.T) {
	testRestoreDelete(t, func() (remote.FS, error) {
		remoteDir := path.Join(defaultBucket, t.TempDir())
		return s3.NewFS(remoteDir)
	})
}
