package fat

import (
	"testing"

	"github.com/rstms/ffs"
)

func TestFileSystemImplementsFileSystem(t *testing.T) {
	var raw interface{}
	raw = new(FileSystem)
	if _, ok := raw.(ffs.FileSystem); !ok {
		t.Fatal("FileSystem should be a FileSystem")
	}
}
