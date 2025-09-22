package image

import (
	//"bytes"
	"bytes"
	"github.com/rstms/ffs"
	"github.com/rstms/ffs/fat"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const MB = 1024 * 1024
const PAD_BYTES = 512

type FileRecord struct {
	Name      string
	ShortName string
	Dir       bool
	Hidden    bool
	System    bool
	ReadOnly  bool
}

type Image struct {
	Filename string
	file     *os.File
	disk     *ffs.FileDisk
	fs       *fat.FileSystem
}

func OpenImage(filename string) (*Image, error) {
	i := Image{Filename: filename}
	var err error
	i.file, err = os.OpenFile(filename, os.O_RDWR, 0600)
	if err != nil {
		return nil, Fatal(err)
	}
	i.disk, err = ffs.NewFileDisk(i.file)
	if err != nil {
		return nil, Fatal(err)
	}
	i.fs, err = fat.New(i.disk)
	if err != nil {
		return nil, Fatal(err)
	}
	return &i, nil
}

func CreateImage(filename, volumeLabel, oemName string, fatType int, size int64) (*Image, error) {
	i := Image{Filename: filename}
	var err error
	err = i.createImageFile(size)
	if err != nil {
		return nil, Fatal(err)
	}
	i.disk, err = ffs.NewFileDisk(i.file)
	if err != nil {
		return nil, Fatal(err)
	}
	err = i.format(fatType, volumeLabel, oemName)
	if err != nil {
		return nil, Fatal(err)
	}
	i.fs, err = fat.New(i.disk)
	if err != nil {
		return nil, Fatal(err)
	}
	return &i, nil
}

func (i *Image) closeFile() error {
	if i.file != nil {
		err := i.file.Close()
		if err != nil {
			return Fatal(err)
		}
		i.file = nil
	}
	return nil
}

func (i *Image) closeDisk() error {
	if i.disk != nil {
		err := i.disk.Close()
		if err != nil {
			return Fatal(err)
		}
		i.disk = nil
	}
	return nil
}

func (i *Image) Close() error {
	defer i.closeDisk()
	defer i.closeFile()
	return nil
}

func (i *Image) ScanFiles() ([]FileRecord, error) {

	ret := []FileRecord{}

	imgRoot, err := i.fs.RootDir()
	if err != nil {
		return ret, Fatal(err)
	}

	records, err := walk("/", imgRoot)
	if err != nil {
		return ret, Fatal(err)
	}

	return records, nil
}

func (i *Image) AddFile(dstPathname, srcPathname string) error {
	//log.Printf("AddFile: dst=%s src=%s\n", dstPathname, srcPathname)
	srcInfo, err := os.Stat(srcPathname)
	if err != nil {
		return Fatal(err)
	}
	dstDir, dstName := filepath.Split(dstPathname)

	dir, err := i.getDir(dstDir)
	if err != nil {
		return Fatal(err)
	}

	entry, err := dir.AddFile(dstName)
	if err != nil {
		return Fatal(err)
	}
	src, err := os.Open(srcPathname)
	if err != nil {
		return Fatal(err)
	}
	defer src.Close()
	dst, err := entry.File()
	if err != nil {
		return Fatal(err)
	}
	defer dst.Close()
	count, err := io.Copy(dst, src)
	if err != nil {
		return Fatal(err)
	}
	if count != srcInfo.Size() {
		return Fatalf("write count mismatch; expected %d, wrote %d\n", srcInfo.Size(), count)
	}
	return nil
}

func MungeImage(dstFilename, srcFilename string, basename string, files []string) error {

	srcFileInfo, err := os.Stat(srcFilename)
	if err != nil {
		return Fatal(err)
	}
	size := srcFileInfo.Size()

	/*
		dstSize := info.Size()
		for _, filename := range files {
			info, err := os.Stat(filename)
			if err != nil {
				return Fatal(err)
			}
			dstSize += info.Size() + int64(PAD_BYTES)
		}
	*/

	srcImage, err := OpenImage(srcFilename)
	if err != nil {
		return Fatal(err)
	}
	defer srcImage.Close()

	volume, err := srcImage.VolumeLabel()
	if err != nil {
		return Fatal(err)
	}
	oem, err := srcImage.OEMName()
	if err != nil {
		return Fatal(err)
	}
	fatType, err := srcImage.FATType()
	if err != nil {
		return Fatal(err)
	}

	dstImage, err := CreateImage(dstFilename, volume, oem, fatType, size)
	if err != nil {
		return Fatal(err)
	}
	defer dstImage.Close()

	records, err := srcImage.ScanFiles()
	if err != nil {
		return Fatal(err)
	}

	for _, record := range records {
		if !record.Dir {
			err := copyFile(dstImage, srcImage, record)
			if err != nil {
				return Fatal(err)
			}
		} else {
			err := dstImage.Mkdir(record.Name)
			if err != nil {
				return Fatal(err)
			}
		}
	}

	for _, file := range files {
		destName, err := filepath.Rel(basename, file)
		if err != nil {
			return Fatal(err)
		}
		err = dstImage.AddFile(destName, file)
		if err != nil {
			return Fatal(err)
		}
	}

	return nil
}

func copyFile(dst, src *Image, record FileRecord) error {
	path, name := filepath.Split(record.Name)
	srcDir, err := src.getDir(path)
	if err != nil {
		return Fatal(err)
	}
	srcEntry := srcDir.Entry(name)
	if srcEntry == nil {
		return Fatalf("src not found: %s", record.Name)
	}
	srcFile, err := srcEntry.File()
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()
	dstDir, err := dst.getDir(path)
	if err != nil {
		return Fatal(err)
	}
	dstEntry, err := dstDir.AddFile(name)
	if err != nil {
		return Fatal(err)
	}
	dstFile, err := dstEntry.File()
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return Fatal(err)
	}
	if record.Hidden {
		err := dstEntry.SetAttr(ffs.AttrHidden, true)
		if err != nil {
			return Fatal(err)
		}
	}
	if record.System {
		err := dstEntry.SetAttr(ffs.AttrSystem, true)
		if err != nil {
			return Fatal(err)
		}
	}
	if record.ReadOnly {
		err := dstEntry.SetAttr(ffs.AttrReadOnly, true)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (i *Image) searchDir(name string) (ffs.Directory, error) {
	dir, err := i.fs.RootDir()
	if err != nil {
		return nil, Fatal(err)
	}
	name = strings.Trim(name, "/")
	if name == "" {
		//log.Println("root exists")
		return dir, nil
	}
	subdirs := strings.Split(name, "/")
	//log.Printf("subdirs: %d %+v\n", len(subdirs), subdirs)
	for _, sub := range subdirs {
		//log.Printf("checking sub[%d]: %s\n", i, sub)
		entry := dir.Entry(sub)
		if entry == nil {
			// no entry present with this name
			//log.Printf("sub=%s not found\n", sub)
			return nil, nil
		}
		if !entry.IsDir() {
			// entry found, but not a directory
			//log.Printf("sub=%s entry=%s not a dir\n", sub, entry.Name())
			return nil, nil
		}
		// step to the next directory
		//log.Printf("sub=%s entry=%s is dir, descending\n", sub, entry.Name())
		dir, err = entry.Dir()
		if err != nil {
			return nil, Fatal(err)
		}
	}
	return dir, nil
}

func (i *Image) getDir(name string) (ffs.Directory, error) {
	dir, err := i.searchDir(name)
	if err != nil {
		return nil, Fatal(err)
	}
	if dir == nil {
		return nil, Fatalf("directory not found: %s", name)
	}
	return dir, nil
}

func (i *Image) IsDir(name string) (bool, error) {
	dir, err := i.searchDir(name)
	if err != nil {
		return false, Fatal(err)
	}
	return dir != nil, nil
}

func (i *Image) Mkdir(pathname string) error {
	//log.Printf("Mkdir: %s\n", pathname)
	exists, err := i.IsDir(pathname)
	if err != nil {
		return Fatal(err)
	}
	if exists {
		return Fatalf("directory exists: %s", pathname)
	}
	dir, name := filepath.Split(pathname)
	parent, err := i.getDir(dir)
	if err != nil {
		return Fatal(err)
	}
	_, err = parent.AddDirectory(name)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

// return total size of files named
func scanFileSizes(filenames []string, pad int64) (int64, error) {
	var size int64
	return size, nil
}

// create, truncate, and reopen the output file
func (i *Image) createImageFile(size int64) error {
	//log.Printf("size before rounding: %d\n", size)
	if size%int64(1024) != 0 {
		size = (size/int64(1024) + 1) * int64(1024)
	}
	//log.Printf("size after rounding: %d\n", size)
	var err error
	i.file, err = os.OpenFile(i.Filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return Fatal(err)
	}
	err = i.file.Truncate(size)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (i *Image) format(fatType int, volumeLabel, oemName string) error {
	var ftype fat.FATType
	switch fatType {
	case 12:
		ftype = fat.FAT12
	case 16:
		ftype = fat.FAT16
	case 32:
		ftype = fat.FAT32
	default:
		return Fatalf("FAT type not 12,16,or 32")
	}
	formatConfig := &fat.SuperFloppyConfig{
		FATType: ftype,
		Label:   volumeLabel,
		OEMName: oemName,
	}
	err := fat.FormatSuperFloppy(i.disk, formatConfig)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func walk(path string, dir ffs.Directory) ([]FileRecord, error) {
	records := []FileRecord{}
	for _, entry := range dir.Entries() {
		switch {
		case entry.Name() == ".":
		case entry.Name() == "..":
		case entry.IsVolumeId():
		default:
			attr := entry.Attr()
			record := FileRecord{
				Name:      filepath.Join(path, entry.Name()),
				ShortName: entry.ShortName(),
				Dir:       attr&ffs.AttrDirectory == ffs.AttrDirectory,
				Hidden:    attr&ffs.AttrHidden == ffs.AttrHidden,
				System:    attr&ffs.AttrSystem == ffs.AttrSystem,
				ReadOnly:  attr&ffs.AttrReadOnly == ffs.AttrReadOnly,
			}
			records = append(records, record)
			if entry.IsDir() {
				subdir, err := entry.Dir()
				if err != nil {
					return []FileRecord{}, Fatal(err)
				}
				subRecords, err := walk(filepath.Join(path, entry.Name()), subdir)
				if err != nil {
					return []FileRecord{}, Fatal(err)
				}
				records = append(records, subRecords...)
			}
		}
	}
	return records, nil
}

func (i *Image) ReadFile(filename string) ([]byte, error) {

	path, name := filepath.Split(filename)

	dir, err := i.getDir(path)

	entry := dir.Entry(name)
	if entry == nil {
		return []byte{}, Fatalf("not found: %s", filename)
	}
	src, err := entry.File()
	if err != nil {
		return []byte{}, Fatal(err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, src)
	if err != nil {
		return []byte{}, Fatal(err)
	}
	return buf.Bytes(), nil
}

// write all files in a directory to the image
func (i *Image) Import(filename string) error {
	err := filepath.WalkDir(filename, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return Fatal(err)
		}
		if path == filename {
			return nil
		}
		dst, err := filepath.Rel(filename, path)
		if err != nil {
			return Fatal(err)
		}
		//log.Printf("dir=%v dst=%s, path=%s\n", d.IsDir(), dst, path)
		if d.IsDir() {
			err := i.Mkdir(dst)
			if err != nil {
				return Fatal(err)
			}
		} else {
			err := i.AddFile(dst, path)
			if err != nil {
				return Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (i *Image) SetAttr(filename string, attr ffs.DirectoryAttr, state bool) error {
	path, file := filepath.Split(filename)
	dir, err := i.getDir(path)
	if err != nil {
		return Fatal(err)
	}
	entry := dir.Entry(file)
	if entry == nil {
		return Fatalf("not found: %s", filename)
	}
	err = entry.SetAttr(attr, state)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (i *Image) GetAttr(filename string) (ffs.DirectoryAttr, error) {
	path, file := filepath.Split(filename)
	dir, err := i.getDir(path)
	if err != nil {
		return 0, Fatal(err)
	}
	entry := dir.Entry(file)
	if entry == nil {
		return 0, Fatalf("not found: %s", filename)
	}
	return entry.Attr(), nil
}

func (i *Image) VolumeLabel() (string, error) {
	vid, err := i.fs.VolumeLabel()
	if err != nil {
		return "", Fatal(err)
	}
	return vid, nil
}

func (i *Image) OEMName() (string, error) {
	oem, err := i.fs.OEMName()
	if err != nil {
		return "", Fatal(err)
	}
	return oem, nil
}

func (i *Image) FATType() (int, error) {
	fatType, err := i.fs.FATType()
	if err != nil {
		return 0, Fatal(err)
	}
	return fatType, nil
}

func (i *Image) Info() (map[string]any, error) {
	info, err := i.fs.Info()
	if err != nil {
		return map[string]any{}, Fatal(err)
	}
	return info, nil
}
