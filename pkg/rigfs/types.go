package rigfs

import (
	"errors"
	"io"
	"io/fs"
)

// ErrCommandFailed is returned when a remote command fails
var ErrCommandFailed = errors.New("command failed")

// Copier is a file-like struct that can copy data to and from io.Reader and io.Writer
type Copier interface {
	CopyFrom(src io.Reader) (int64, error)
	CopyTo(dst io.Writer) (int64, error)
}

// File is a file in the remote filesystem
type File interface {
	fs.File
	io.Seeker
	io.ReadCloser
	io.Writer
	Copier
}

// Fsys is a filesystem on the remote host
type Fsys interface {
	fs.FS
	OpenFile(path string, flag int, perm fs.FileMode) (File, error)
	Sha256(path string) (string, error)
	Stat(path string) (fs.FileInfo, error)
	Remove(path string) error
	RemoveAll(path string) error
	MkDirAll(path string, perm fs.FileMode) error
}
