package file

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFile_UploadDownloadDelete(t *testing.T) {
	root := t.TempDir()
	f := NewLocalFile(root)
	ctx := context.Background()

	content := []byte("hello world")
	if err := f.Upload(ctx, "test/a.txt", bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	// Verify file exists on disk.
	if _, err := os.Stat(filepath.Join(root, "test/a.txt")); err != nil {
		t.Fatalf("uploaded file not found: %v", err)
	}

	rc, err := f.Download(ctx, "test/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", string(got))
	}

	if err := f.Delete(ctx, "test/a.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "test/a.txt")); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestLocalFile_DownloadNotFound(t *testing.T) {
	root := t.TempDir()
	f := NewLocalFile(root)
	ctx := context.Background()

	_, err := f.Download(ctx, "nonexistent.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLocalFile_NestedDirs(t *testing.T) {
	root := t.TempDir()
	f := NewLocalFile(root)
	ctx := context.Background()

	if err := f.Upload(ctx, "a/b/c/d.txt", bytes.NewReader([]byte("nested"))); err != nil {
		t.Fatal(err)
	}

	rc, err := f.Download(ctx, "a/b/c/d.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "nested" {
		t.Errorf("expected %q, got %q", "nested", string(got))
	}
}
