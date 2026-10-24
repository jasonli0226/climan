package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

const (
	FileName = "registry.yaml"
	lockName = FileName + ".lock"

	defaultLockWait    = 5 * time.Second
	defaultRenameRetry = 2 * time.Second
	lockPoll           = 50 * time.Millisecond
	renamePoll         = 100 * time.Millisecond
)

var (
	// ErrLocked means another climan process holds the registry lock.
	ErrLocked = errors.New("registry is locked by another climan process")
	// ErrFileBusy means the registry could not be replaced because another
	// program has it open (Windows).
	ErrFileBusy = errors.New("registry is open in another program")
)

// Store reads and writes the registry file in Dir.
type Store struct {
	Dir         string        // config dir
	LockWait    time.Duration // default 5s
	RenameRetry time.Duration // default 2s
}

// Path returns the registry file path.
func (s Store) Path() string { return filepath.Join(s.Dir, FileName) }

// Load reads the registry without locking. A missing file is an empty
// registry.
func (s Store) Load() (*Registry, error) {
	data, err := os.ReadFile(s.Path())
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(s.Path(), data)
}

// Update locks the registry, loads it, applies fn and writes the result
// atomically. Nothing is written when fn or the load fails.
func (s Store) Update(fn func(*Registry) error) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()

	reg, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(reg); err != nil {
		return err
	}
	data, err := reg.Marshal()
	if err != nil {
		return err
	}
	return s.write(data)
}

func (s Store) lock() (func(), error) {
	wait := s.LockWait
	if wait == 0 {
		wait = defaultLockWait
	}
	path := filepath.Join(s.Dir, lockName)
	l := flock.New(path)
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	ok, err := l.TryLockContext(ctx, lockPoll)
	if !ok {
		if err == nil || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w (lock file %s)", ErrLocked, path)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() { _ = l.Unlock() }, nil
}

// write replaces the registry file via a synced temp file and rename. If the
// rename keeps failing, the temp file is kept so no changes are lost.
func (s Store) write(data []byte) error {
	tmp, err := os.CreateTemp(s.Dir, "registry-*.yaml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return err
	}

	retry := s.RenameRetry
	if retry == 0 {
		retry = defaultRenameRetry
	}
	deadline := time.Now().Add(retry)
	for {
		err = os.Rename(name, s.Path())
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %v; changes saved to %s", ErrFileBusy, err, name)
		}
		time.Sleep(renamePoll)
	}
}
