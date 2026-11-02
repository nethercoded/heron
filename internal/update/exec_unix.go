//go:build !windows

package update

import (
	"os"
	"syscall"
)

func Exec(path string) error {
	return syscall.Exec(path, append([]string{path}, os.Args[1:]...), os.Environ())
}
