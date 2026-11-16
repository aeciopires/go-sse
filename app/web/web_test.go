package web

import (
	"io/fs"
	"testing"
)

func TestEmbeddedAssets(t *testing.T) {
	for _, tc := range []struct {
		fsys fs.FS
		name string
	}{
		{Templates(), "index.html"},
		{Static(), "app.js"},
		{Static(), "style.css"},
		{Static(), "theme.js"},
	} {
		if _, err := fs.Stat(tc.fsys, tc.name); err != nil {
			t.Errorf("embedded file %s missing: %v", tc.name, err)
		}
	}
}

func TestMustSubPanicsOnInvalidPath(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustSub with invalid path did not panic")
		}
	}()
	mustSub("../invalid")
}
