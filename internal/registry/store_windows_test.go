//go:build windows

package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestUpdateRenameBusy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := syscall.UTF16PtrFromString(path)
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)

	err = Store{Dir: dir, RenameRetry: 200 * time.Millisecond}.Update(func(r *Registry) error {
		r.Tools["rg"] = &Entry{Status: Pending, Source: Discovered}
		return nil
	})
	if !errors.Is(err, ErrFileBusy) {
		t.Fatalf("err = %v, want ErrFileBusy", err)
	}
	_, tmp, ok := strings.Cut(err.Error(), "changes saved to ")
	if !ok {
		t.Fatalf("err %q does not name the temp file", err)
	}
	if _, statErr := os.Stat(tmp); statErr != nil {
		t.Fatalf("temp file %s missing: %v", tmp, statErr)
	}
}
