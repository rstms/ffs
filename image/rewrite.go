/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package image

import (
	"github.com/rstms/ffs"
	"os"
	"os/exec"
	"path/filepath"
)

func RewriteImage(dstFile, srcFile string, fatType int, size int64) error {
	src, err := OpenImage(srcFile)
	if err != nil {
		return Fatal(err)
	}
	defer src.Close()

	volume, err := src.VolumeLabel()
	if err != nil {
		return Fatal(err)
	}
	oem, err := src.OEMName()
	if err != nil {
		return Fatal(err)
	}

	records, err := src.ScanFiles()
	if err != nil {
		return Fatal(err)
	}
	tempDir, err := os.MkdirTemp("", "mkimage-*")
	if err != nil {
		return Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	for _, record := range records {
		if record.Dir {
			err := os.Mkdir(filepath.Join(tempDir, record.Name), 0700)
			if err != nil {
				return Fatal(err)
			}
		} else {
			dstFile := filepath.Join(tempDir, record.Name)
			output, err := exec.Command("mtype", "-i", srcFile, "::"+record.Name).Output()
			if err != nil {
				return Fatal(err)
			}
			err = os.WriteFile(dstFile, output, 0600)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	dst, err := CreateImage(dstFile, volume, oem, fatType, size)
	if err != nil {
		return Fatal(err)
	}
	defer dst.Close()
	err = dst.Import(tempDir)
	if err != nil {
		return Fatal(err)
	}
	for _, record := range records {
		if record.System {
			err := dst.SetAttr(record.Name, ffs.AttrSystem, true)
			if err != nil {
				return Fatal(err)
			}
		}
		if record.Hidden {
			err := dst.SetAttr(record.Name, ffs.AttrHidden, true)
			if err != nil {
				return Fatal(err)
			}
		}
		if record.ReadOnly {
			err := dst.SetAttr(record.Name, ffs.AttrReadOnly, true)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	return nil
}
