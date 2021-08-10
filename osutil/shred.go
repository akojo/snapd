package osutil

import (
	"fmt"
	"io"
	"os"
)

func Shred(path string) error {
	random, err := openfile("/dev/urandom", os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	defer random.Close()
	for i := 0; i < 10; i = i + 1 {
		dst, err := openfile(path, os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("unable to open %s: %v", path, err)
		}
		defer dst.Close()
		info, err := dst.Stat()
		if err != nil {
			return fmt.Errorf("unable to stat %s: %v", path, err)
		}
		if _, err := io.CopyN(dst, random, info.Size()); err != nil {
			return fmt.Errorf("unable to write %s: %v", path, err)
		}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("unable to remove %s: %v", path, err)
	}
	return nil
}
