package main

import (
	"io/fs"
	"mime"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryEmbeddedFileHasAMIMEType(t *testing.T) {
	// given
	root := "static"

	// when
	var untyped []string
	err := fs.WalkDir(staticFiles, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return nil
		}

		if mime.TypeByExtension(path.Ext(name)) == "" {
			untyped = append(untyped, name)
		}

		return nil
	})

	// then
	require.NoError(t, err)
	assert.Empty(t, untyped, "these embedded files have an extension with no MIME type; register it in the init in internal/controllers/static_controller.go:\n%s", strings.Join(untyped, "\n"))
}
