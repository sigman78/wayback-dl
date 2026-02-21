package wayback

import (
	"io"
	"os"
	"path/filepath"
)

// Storage abstracts reading and writing downloaded snapshot files.
// Logical paths are forward-slash relative paths as returned by URLToLocalPath
// (e.g. "example.com/page/index.html"). Implementations map them to wherever
// files actually live (OS directory, zip archive, memory map, …).
type Storage interface {
	// Exists reports whether the logical path already has content.
	Exists(path string) bool
	// Put writes the content of r to path. The write is atomic —
	// no partial file is visible to concurrent readers.
	Put(path string, r io.Reader) error
	// Get returns the full content of path.
	Get(path string) ([]byte, error)
	// PutBytes writes data to path (convenience wrapper around Put).
	PutBytes(path string, data []byte) error
}

// LocalStorage is the default Storage implementation that mirrors the
// logical layout into a root directory on the OS filesystem.
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage returns a LocalStorage rooted at dir.
// The root directory is created lazily by Put/PutBytes.
func NewLocalStorage(dir string) *LocalStorage {
	return &LocalStorage{rootDir: dir}
}

// abs converts a logical forward-slash path to an absolute OS path.
func (s *LocalStorage) abs(path string) string {
	return filepath.Join(s.rootDir, filepath.FromSlash(path))
}

// Exists reports whether path already exists in storage.
func (s *LocalStorage) Exists(path string) bool {
	_, err := os.Stat(s.abs(path))
	return err == nil
}

// Put streams r into path atomically via a temp file + rename.
func (s *LocalStorage) Put(path string, r io.Reader) error {
	fullPath := s.abs(path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, ".wbdl-*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName) // no-op if already renamed
	}()
	if _, err := io.Copy(tmpFile, r); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, fullPath) //nolint:gosec // G703: fullPath is sanitized by URLToLocalPath
}

// Get returns the full content of path.
func (s *LocalStorage) Get(path string) ([]byte, error) {
	return os.ReadFile(s.abs(path)) //nolint:gosec // G304: path is written by this program
}

// PutBytes writes data to path, creating parent directories as needed.
func (s *LocalStorage) PutBytes(path string, data []byte) error {
	fullPath := s.abs(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0750); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0600)
}
