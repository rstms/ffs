package ffs

// A FileSystem provides access to a tree hierarchy of directories
// and files.
type FileSystem interface {
	// RootDir returns the single root directory.
	RootDir() (Directory, error)
	Info() (map[string]any, error)
	FATType() (int, error)
	OEMName() (string, error)
	VolumeLabel() (string, error)
}
