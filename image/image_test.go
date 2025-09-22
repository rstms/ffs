package image

import (
	"github.com/rstms/ffs"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mdir(t *testing.T, filename string) {
	cmd := exec.Command("mdir", "-i", filename)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	require.Nil(t, err)
}

func cp(t *testing.T, src, dst string) {
	err := exec.Command("cp", src, dst).Run()
	require.Nil(t, err)
}

func rm(t *testing.T, filename string) {
	err := exec.Command("rm", "-f", filename).Run()
	require.Nil(t, err)
}

func TestImageListFiles(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(srcFile)
	require.Nil(t, err)
	defer i.Close()
	records, err := i.ScanFiles()
	require.Nil(t, err)
	for _, record := range records {
		var attrs string
		if record.Dir {
			attrs += "d"
		}
		if record.ReadOnly {
			attrs += "r"
		}
		if record.Hidden {
			attrs += "h"
		}
		if record.System {
			attrs += "s"
		}
		log.Printf("%s shortName=%s attrs=%s\n", record.Name, record.ShortName, attrs)
	}
}

func testFiles() []string {
	return []string{
		filepath.Join("testdata", "foo"),
		filepath.Join("testdata", "bar"),
		filepath.Join("testdata", "baz"),
	}
}

func TestImageAddFiles(t *testing.T) {
	dstFile := filepath.Join("testdata", "dst.img")
	i, err := CreateImage(dstFile, "add", "ffs", 12, 1440*1024)
	require.Nil(t, err)
	for _, file := range testFiles() {
		_, name := filepath.Split(file)
		err := i.AddFile(name, file)
		require.Nil(t, err)
	}

	err = i.Mkdir("files")
	require.Nil(t, err)

	newFile := filepath.Join("testdata", "howdy")
	err = os.WriteFile(newFile, []byte("howdy howdy howdy"), 0600)
	require.Nil(t, err)

	err = i.AddFile(filepath.Join("files", "howdy"), newFile)
	require.Nil(t, err)

	i.Close()
	log.Println("after")
	mdir(t, dstFile)
}

func TestImageMungeNoFiles(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	rewriteFile := filepath.Join("testdata", "rewrite.img")
	rm(t, rewriteFile)
	err := RewriteImage(rewriteFile, srcFile, 12, 2880*1024)
	require.Nil(t, err)
	dstFile := filepath.Join("testdata", "munged.img")
	rm(t, dstFile)

	err = MungeImage(dstFile, rewriteFile, "testdata", []string{})
	require.Nil(t, err)
	mdir(t, dstFile)
}

func TestImageMungeFiles(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	rewriteFile := filepath.Join("testdata", "rewrite.img")
	rm(t, rewriteFile)
	err := RewriteImage(rewriteFile, srcFile, 12, 2880*1024)
	require.Nil(t, err)
	dstFile := filepath.Join("testdata", "munged.img")
	rm(t, dstFile)

	err = MungeImage(dstFile, rewriteFile, "testdata", testFiles())
	require.Nil(t, err)
	mdir(t, dstFile)
}

func TestImageIsDir(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(srcFile)
	require.Nil(t, err)
	ret, err := i.IsDir("/")
	require.Nil(t, err)
	require.True(t, ret)

	ret, err = i.IsDir("/foo")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("foo")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("foo/bar/baz")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("syslinux.cfg")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("IPXE")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("EFI")
	require.Nil(t, err)
	require.True(t, ret)

	ret, err = i.IsDir("EFI/foo")
	require.Nil(t, err)
	require.False(t, ret)

	ret, err = i.IsDir("EFI/BOOT")
	require.Nil(t, err)
	require.True(t, ret)

	ret, err = i.IsDir("EFI/BOOT/GROOT")
	require.Nil(t, err)
	require.False(t, ret)

}

func TestImageMkdir(t *testing.T) {
	dstFile := filepath.Join("testdata", "dst.img")

	i, err := CreateImage(dstFile, "mkdir", "ffs", 12, 1440*1024)
	require.Nil(t, err)

	ret, err := i.IsDir("/foo")
	require.Nil(t, err)
	require.False(t, ret)

	err = i.Mkdir("/foo")
	require.Nil(t, err)

	ret, err = i.IsDir("/foo")
	require.Nil(t, err)
	require.True(t, ret)

	i.Close()

	mdir(t, dstFile)

	j, err := OpenImage(dstFile)
	require.Nil(t, err)

	err = j.Mkdir("/foo/bar")
	require.Nil(t, err)

	ret, err = j.IsDir("foo/bar")
	require.Nil(t, err)
	require.True(t, ret)
	j.Close()

	mdir(t, dstFile)
}

func TestImageRewrite(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	dstFile := filepath.Join("testdata", "dst.img")
	err := RewriteImage(dstFile, srcFile, 12, 2880*1024)
	require.Nil(t, err)
	mdir(t, dstFile)

	i, err := OpenImage(dstFile)
	require.Nil(t, err)

	volume, err := i.VolumeLabel()
	require.Nil(t, err)
	require.IsType(t, "", volume)
	require.NotEmpty(t, volume)
	log.Printf("volume=%s\n", volume)

	oem, err := i.OEMName()
	require.Nil(t, err)
	require.IsType(t, "", oem)
	require.NotEmpty(t, oem)
	log.Printf("oem=%s\n", oem)
}

func TestImageImport(t *testing.T) {
	imgFile := filepath.Join("testdata", "import.img")
	rm(t, imgFile)
	i, err := CreateImage(imgFile, "import", "ffs", 12, 2880*1024)
	importPath := filepath.Join("testdata", "files")
	err = i.Import(importPath)
	require.Nil(t, err)

	for _, file := range testFiles() {
		_, name := filepath.Split(file)
		err := i.AddFile(name, file)
		require.Nil(t, err)
	}

	i.Close()
	mdir(t, imgFile)

	i, err = OpenImage(imgFile)
	require.Nil(t, err)
	err = i.SetAttr("foo", ffs.AttrHidden, true)
	i.Close()
	mdir(t, imgFile)

	i, err = OpenImage(imgFile)
	require.Nil(t, err)
	err = i.SetAttr("foo", ffs.AttrHidden, false)
	require.Nil(t, err)
	i.Close()
	mdir(t, imgFile)
}

func TestImageVolumeLabel(t *testing.T) {
	imgFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(imgFile)
	require.Nil(t, err)
	volume, err := i.VolumeLabel()
	require.Nil(t, err)
	require.IsType(t, "", volume)
	log.Printf("volume Label = '%s'\n", volume)
}

func TestImageOEMName(t *testing.T) {
	imgFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(imgFile)
	require.Nil(t, err)
	oem, err := i.OEMName()
	require.Nil(t, err)
	require.IsType(t, "", oem)
	log.Printf("OEM Name = '%s'\n", oem)
}

func TestImageFATType(t *testing.T) {
	imgFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(imgFile)
	require.Nil(t, err)
	fatType, err := i.FATType()
	require.Nil(t, err)
	require.IsType(t, int(0), fatType)
	log.Printf("FAT type = FAT%d\n", fatType)
}

func TestImageInfo(t *testing.T) {
	imgFile := filepath.Join("testdata", "src.img")
	i, err := OpenImage(imgFile)
	require.Nil(t, err)
	info, err := i.Info()
	require.Nil(t, err)
	for key, value := range info {
		log.Printf("%s: %v\n", key, value)
	}
}

func TestImageReadFile(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	dstFile := filepath.Join("testdata", "dst.img")

	err := RewriteImage(dstFile, srcFile, 12, 2880*1024)
	require.Nil(t, err)

	i, err := OpenImage(dstFile)
	require.Nil(t, err)
	defer i.Close()

	data, err := i.ReadFile("/autoexec.ipxe")
	require.Nil(t, err)

	log.Printf("%s\n", string(data))
}
