package fat

import (
	"encoding/json"
	"github.com/rstms/ffs"
	"strings"
)

// FileSystem is the implementation of ffs.FileSystem that can read a
// FAT filesystem.
type FileSystem struct {
	bs      *BootSectorCommon
	device  ffs.BlockDevice
	fat     *FAT
	rootDir *DirectoryCluster
}

var _ ffs.FileSystem = (*FileSystem)(nil)

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

func (f *FileSystem) Info() (map[string]any, error) {
	var ret map[string]any
	bs, err := DecodeBootSector(f.device)
	if err != nil {
		return ret, Fatal(err)
	}
	data, err := json.Marshal(bs)
	if err != nil {
		return ret, Fatal(err)
	}
	err = json.Unmarshal(data, &ret)
	if err != nil {
		return ret, Fatal(err)
	}
	return ret, nil
}

func (f *FileSystem) FATType() (int, error) {
	bs, err := DecodeBootSector(f.device)
	if err != nil {
		return 0, Fatal(err)
	}
	switch bs.FATType() {
	case FAT12:
		return 12, nil
	case FAT16:
		return 16, nil
	case FAT32:
		return 32, nil
	}
	return 0, Fatalf("unexpected FAT type")
}

func (f *FileSystem) OEMName() (string, error) {
	bs, err := DecodeBootSector(f.device)
	if err != nil {
		return "", Fatal(err)
	}
	return bs.OEMName, nil
}

func (f *FileSystem) VolumeLabel() (string, error) {
	bs, err := DecodeBootSector(f.device)
	if err != nil {
		return "", Fatal(err)
	}
	label, err := DecodeVolumeLabel(f.device, bs.FATType())
	if err != nil {
		return "", Fatal(err)
	}
	return strings.TrimSpace(label), nil
}
