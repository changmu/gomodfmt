package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWriteFileAtomic_WritesCorrectBytes verifies the basic write contract.
func TestWriteFileAtomic_WritesCorrectBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	want := []byte("rewritten content\n")
	if err := writeFileAtomic(path, want, 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("file content = %q, want %q", got, want)
	}
}

// TestWriteFileAtomic_PreservesMode verifies the file mode is honored.
func TestWriteFileAtomic_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("seed\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := writeFileAtomic(path, []byte("new\n"), 0o600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0o600", info.Mode().Perm())
	}
}

// TestWriteFileAtomic_NoLeftoverTempFiles ensures successful writes leave no
// .gomodfmt-* temp file behind in the target directory.
func TestWriteFileAtomic_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := writeFileAtomic(path, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "go.mod" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}

// TestWriteFileAtomic_UsesRename verifies the implementation uses rename
// rather than in-place truncate+write. This is the proxy contract for
// atomicity: after a successful atomic write the file inode must differ
// from before (rename produces a new inode), proving the old file was
// never truncated mid-write.
func TestWriteFileAtomic_UsesRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	origStat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat orig: %v", err)
	}
	origIno := origStat.Sys().(*syscall.Stat_t).Ino

	if err := writeFileAtomic(path, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	newStat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat new: %v", err)
	}
	newIno := newStat.Sys().(*syscall.Stat_t).Ino

	if origIno == newIno {
		t.Errorf("inode did not change (orig=%d new=%d): writeFileAtomic must use rename to be crash-safe", origIno, newIno)
	}
}

// TestWriteFileAtomic_TempInSameDir ensures the temp file is created in the
// target file's directory (so rename is atomic — cross-filesystem renames
// fall back to copy+delete and lose atomicity).
func TestWriteFileAtomic_TempInSameDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Make dir read-only (write bit removed) so creating a temp file in dir
	// would fail. If writeFileAtomic creates the temp elsewhere (e.g.,
	// os.TempDir), the write would succeed despite read-only dir — that
	// would be a bug because cross-fs rename is not atomic.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	err := writeFileAtomic(path, []byte("new\n"), 0o644)
	if err == nil {
		t.Errorf("expected error when target dir is read-only (temp file should be in same dir); writeFileAtomic must not use os.TempDir")
	}
}
