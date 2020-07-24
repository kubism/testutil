/*
Copyright 2020 Testutil Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kubism/testutil/pkg/rand"
)

// TODO: add FileOption to allow users to properly set permissions for files

type TempFile struct {
	// Path of the temporary file
	Path string
}

// NewTempFile creates a new temporary file in a temporary directory.
// The provided content is written to the file.
// Make sure to always call Close once the file is not required anymore.
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

// Close will remove the temporary directory containing the file.
func (f *TempFile) Close() error {
	return os.RemoveAll(filepath.Dir(f.Path))
}

type TempDir struct {
	// Path to the temporary directory
	Path string
}

// NewTempDir creates a new temporary directory.
func NewTempDir() (*TempDir, error) {
	path, err := ioutil.TempDir("", "kubism-testutil")
	if err != nil {
		return nil, err
	}
	return &TempDir{
		Path: path,
	}, nil
}

// Close will remove the temporary directory.
func (d *TempDir) Close() error {
	return os.RemoveAll(d.Path)
}
