// go-common local proxy functions

package image

import (
	"github.com/rstms/go-common"
)

func Fatal(err error) error {
	return common.Fatal(err)
}

func Fatalf(format string, args ...interface{}) error {
	return common.Fatalf(format, args...)
}

func IsFile(filename string) bool {
	return common.IsFile(filename)
}
