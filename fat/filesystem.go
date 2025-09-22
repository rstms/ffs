package fat

import (
	"github.com/rstms/ffs"
)

// FileSystem is the implementation of ffs.FileSystem that can read a
// FAT filesystem.
type FileSystem struct {
	bs      *BootSectorCommon
	device  ffs.BlockDevice
	fat     *FAT
	rootDir *DirectoryCluster
}

// New returns a new FileSystem for accessing a previously created
// FAT filesystem.
func New(device ffs.BlockDevice) (*FileSystem, error) {
	bs, err := DecodeBootSector(device)
	if err != nil {
		return nil, Fatal(err)
	}

	fat, err := DecodeFAT(device, bs, 0)
	if err != nil {
		return nil, Fatal(err)
	}

	var rootDir *DirectoryCluster
	if bs.FATType() == FAT32 {
		// WARNING: very experimental and incomplete
		rootDir, err = DecodeFAT32RootDirectoryCluster(device, fat)
		if err != nil {
			return nil, Fatal(err)
		}
	} else {
		rootDir, err = DecodeFAT16RootDirectoryCluster(device, bs)
		if err != nil {
			return nil, Fatal(err)
		}
	}

	result := &FileSystem{
		bs:      bs,
		device:  device,
		fat:     fat,
		rootDir: rootDir,
	}

	return result, nil
}

func (f *FileSystem) RootDir() (ffs.Directory, error) {
	dir := &Directory{
		device:     f.device,
		dirCluster: f.rootDir,
		fat:        f.fat,
	}

	return dir, nil
}
