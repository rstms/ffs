package fat

import (
	"fmt"
	"strings"
	"time"

	"github.com/rstms/ffs"
)

// Directory implements ffs.Directory and is used to interface with
// a directory on a FAT filesystem.
type Directory struct {
	device     ffs.BlockDevice
	dirCluster *DirectoryCluster
	fat        *FAT
}

// ensure Directory implements ffs.Directory
var _ ffs.Directory = (*Directory)(nil)

// DirectoryEntry implements ffs.DirectoryEntry and represents a single
// file/folder within a directory in a FAT filesystem. Note that there may be
// more than one underlying directory entry data structure on the disk to
// account for long filenames.
type DirectoryEntry struct {
	dir        *Directory
	lfnEntries []*DirectoryClusterEntry
	entry      *DirectoryClusterEntry

	name string
}

// ensure DirectoryEntry implements ffs.DirectoryEntry
var _ ffs.DirectoryEntry = (*DirectoryEntry)(nil)

// DecodeDirectoryEntry takes a list of entries, decodes the next full
// DirectoryEntry, and returns the newly created entry, the remaining
// entries, and an error, if there was one.
func DecodeDirectoryEntry(d *Directory, entries []*DirectoryClusterEntry) (*DirectoryEntry, []*DirectoryClusterEntry, error) {
	var lfnEntries []*DirectoryClusterEntry
	var entry *DirectoryClusterEntry
	var name string

	// Skip all the deleted entries
	for len(entries) > 0 && entries[0].deleted {
		entries = entries[1:]
	}

	// Skip the volume ID
	if len(entries) > 0 && entries[0].IsVolumeId() {
		entries = entries[1:]
	}

	if len(entries) == 0 {
		return nil, entries, nil
	}

	// We have a long entry, so we have to traverse to the point where
	// we're done. Also, calculate out the name and such.
	if entries[0].IsLong() {
		lfnEntries = make([]*DirectoryClusterEntry, 0, 3)
		for entries[0].IsLong() {
			lfnEntries = append(lfnEntries, entries[0])
			entries = entries[1:]
		}

		var nameBytes []rune
		nameBytes = make([]rune, 0, 13*len(lfnEntries))
		for i := len(lfnEntries) - 1; i >= 0; i-- {
			for _, char := range lfnEntries[i].longName {
				nameBytes = append(nameBytes, char)
			}
		}

		name = string(nameBytes)
	}

	// Get the short entry
	entry = entries[0]
	entries = entries[1:]

	// If the short entry is deleted, ignore everything
	if entry.deleted {
		return nil, entries, nil
	}

	if name == "" {
		name = strings.TrimSpace(entry.name)
		ext := strings.TrimSpace(entry.ext)
		if ext != "" {
			name = fmt.Sprintf("%s.%s", name, ext)
		}
	}

	result := &DirectoryEntry{
		dir:        d,
		lfnEntries: lfnEntries,
		entry:      entry,
		name:       name,
	}

	return result, entries, nil
}

func (d *DirectoryEntry) Dir() (ffs.Directory, error) {
	if !d.IsDir() {
		panic("not a directory")
	}

	dirCluster, err := DecodeDirectoryCluster(
		d.entry.cluster, d.dir.device, d.dir.fat)
	if err != nil {
		return nil, Fatal(err)
	}

	result := &Directory{
		device:     d.dir.device,
		dirCluster: dirCluster,
		fat:        d.dir.fat,
	}

	return result, nil
}

func (d *DirectoryEntry) File() (ffs.File, error) {
	if d.IsDir() {
		panic("not a file")
	}

	result := &File{
		chain: &ClusterChain{
			device:       d.dir.device,
			fat:          d.dir.fat,
			startCluster: d.entry.cluster,
		},
		dir:   d.dir,
		entry: d.entry,
	}

	return result, nil
}

func (d *DirectoryEntry) IsDir() bool {
	return (d.entry.attr & ffs.AttrDirectory) == ffs.AttrDirectory
}

func (d *DirectoryEntry) IsVolumeId() bool {
	return (d.entry.attr & ffs.AttrVolumeId) == ffs.AttrVolumeId
}

func (d *DirectoryEntry) Name() string {
	return d.name
}

func (d *DirectoryEntry) Attr() ffs.DirectoryAttr {
	return d.entry.attr
}

func (d *DirectoryEntry) SetAttr(attr ffs.DirectoryAttr, state bool) error {
	switch attr {
	case ffs.AttrHidden:
	case ffs.AttrSystem:
	case ffs.AttrReadOnly:
	default:
		return Fatalf("unsettable attribute")
	}
	if state {
		d.entry.attr |= attr
	} else {
		d.entry.attr &= ^attr
	}
	err := d.dir.dirCluster.WriteToDevice(d.dir.device, d.dir.fat)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (d *DirectoryEntry) SetReadOnly(state bool) error {
	err := d.SetAttr(ffs.AttrReadOnly, state)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (d *DirectoryEntry) SetSystem(state bool) error {
	err := d.SetAttr(ffs.AttrSystem, state)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (d *DirectoryEntry) SetHidden(state bool) error {
	err := d.SetAttr(ffs.AttrHidden, state)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (d *DirectoryEntry) IsReadOnly() bool {
	return d.entry.attr&ffs.AttrReadOnly == ffs.AttrReadOnly
}

func (d *DirectoryEntry) IsSystem() bool {
	return d.entry.attr&ffs.AttrSystem == ffs.AttrSystem
}

func (d *DirectoryEntry) IsHidden() bool {
	return d.entry.attr&ffs.AttrHidden == ffs.AttrHidden
}

func (d *DirectoryEntry) ShortName() string {
	if d.entry.name == "." || d.entry.name == ".." {
		return d.entry.name
	}

	return fmt.Sprintf("%s.%s", d.entry.name, d.entry.ext)
}

func (d *Directory) AddDirectory(name string) (ffs.DirectoryEntry, error) {
	entry, err := d.addEntry(name, ffs.AttrDirectory)
	if err != nil {
		return nil, Fatal(err)
	}

	// Create the new directory cluster
	newDirCluster := NewDirectoryCluster(
		entry.entry.cluster, d.dirCluster.startCluster, entry.entry.createTime)

	if err := newDirCluster.WriteToDevice(d.device, d.fat); err != nil {
		return nil, Fatal(err)
	}

	return entry, nil
}

func (d *Directory) AddFile(name string) (ffs.DirectoryEntry, error) {
	entry, err := d.addEntry(name, ffs.DirectoryAttr(0))
	if err != nil {
		return nil, Fatal(err)
	}

	return entry, nil
}

func (d *Directory) Entries() []ffs.DirectoryEntry {
	entries := d.dirCluster.entries
	result := make([]ffs.DirectoryEntry, 0, len(entries)/2)
	for len(entries) > 0 {
		var entry *DirectoryEntry
		entry, entries, _ = DecodeDirectoryEntry(d, entries)
		if entry != nil {
			result = append(result, entry)
		}
	}

	return result
}

func (d *Directory) Entry(name string) ffs.DirectoryEntry {
	name = strings.ToUpper(name)

	for _, entry := range d.Entries() {
		if strings.ToUpper(entry.Name()) == name {
			return entry
		}
	}

	return nil
}

func (d *Directory) addEntry(name string, attr ffs.DirectoryAttr) (*DirectoryEntry, error) {
	name = strings.TrimSpace(name)

	entries := d.Entries()
	usedNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.ToUpper(entry.Name()) == strings.ToUpper(name) {
			return nil, Fatalf("name already exists: %s", name)
		}

		// Add it to the list of used names
		dirEntry := entry.(*DirectoryEntry)
		usedNames = append(usedNames, dirEntry.ShortName())
	}

	shortName, err := generateShortName(name, usedNames)
	if err != nil {
		return nil, Fatal(err)
	}

	var lfnEntries []*DirectoryClusterEntry
	if shortName != strings.ToUpper(name) {
		lfnEntries, err = NewLongDirectoryClusterEntry(name, shortName)
		if err != nil {
			return nil, Fatal(err)
		}
	}

	// Allocate space for a cluster
	startCluster, err := d.fat.AllocChain()
	if err != nil {
		return nil, Fatal(err)
	}

	createTime := time.Now()

	// Create the entry for the short name
	shortParts := strings.Split(shortName, ".")
	if len(shortParts) == 1 {
		shortParts = append(shortParts, "")
	}

	shortEntry := new(DirectoryClusterEntry)
	shortEntry.attr = attr
	shortEntry.name = shortParts[0]
	shortEntry.ext = shortParts[1]
	shortEntry.cluster = startCluster
	shortEntry.accessTime = createTime
	shortEntry.createTime = createTime
	shortEntry.writeTime = createTime

	// Write the new FAT out
	if err := d.fat.WriteToDevice(d.device); err != nil {
		return nil, Fatal(err)
	}

	// Write the entries out in this directory
	if lfnEntries != nil {
		d.dirCluster.entries = append(d.dirCluster.entries, lfnEntries...)
	}
	d.dirCluster.entries = append(d.dirCluster.entries, shortEntry)

	if err := d.dirCluster.WriteToDevice(d.device, d.fat); err != nil {
		return nil, Fatal(err)
	}

	newEntry := &DirectoryEntry{
		dir:        d,
		lfnEntries: lfnEntries,
		entry:      shortEntry,
	}

	return newEntry, nil
}
