package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kubism/testutil/pkg/rand"
)

type TempFile struct {
	Path string
}

// TODO: add CreateOptions, e.g. for permissions
func NewTempFile(content []byte) (*TempFile, error) {
	dir, err := ioutil.TempDir("", "kubism-testutil")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, rand.String(5))
	err = ioutil.WriteFile(path, content, 0644)
	if err != nil {
		return nil, err
	}
	return &TempFile{
		Path: path,
	}, nil
}

func (f *TempFile) Close() error {
	return os.RemoveAll(filepath.Dir(f.Path))
}

type TempDir struct {
	Path string
}

func NewTempDir() (*TempDir, error) {
	path, err := ioutil.TempDir("", "kubism-testutil")
	if err != nil {
		return nil, err
	}
	return &TempDir{
		Path: path,
	}, nil
}

func (d *TempDir) Close() error {
	return os.RemoveAll(d.Path)
}
