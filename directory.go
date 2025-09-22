package ffs

type DirectoryAttr uint8

const (
	AttrReadOnly  DirectoryAttr = 0x01
	AttrHidden                  = 0x02
	AttrSystem                  = 0x04
	AttrVolumeId                = 0x08
	AttrDirectory               = 0x10
	AttrArchive                 = 0x20
	AttrLongName                = AttrReadOnly | AttrHidden | AttrSystem | AttrVolumeId
)

// Directory is an entry in a filesystem that stores files.
type Directory interface {
	Entry(name string) DirectoryEntry
	Entries() []DirectoryEntry
	AddDirectory(name string) (DirectoryEntry, error)
	AddFile(name string) (DirectoryEntry, error)
}

// DirectoryEntry represents a single entry within a directory,
// which can be either another Directory or a File.
type DirectoryEntry interface {
	Name() string
	ShortName() string
	IsDir() bool
	Dir() (Directory, error)
	File() (File, error)
	IsVolumeId() bool
	Attr() DirectoryAttr
	SetAttr(DirectoryAttr, bool) error
}
