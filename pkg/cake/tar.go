package cake

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/facebookgo/symwalk"
)

// tar util with symlink traversal support
func Tar(source string, target string) error {

	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("Unable to tar files - %v", err.Error())
	}

	file, err := os.Create(target)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not create tarball file '%s', got error '%s'", target, err.Error()))
	}

	writer := tar.NewWriter(file)
	defer writer.Close()

	return symwalk.Walk(source, func(file string, fi os.FileInfo, err error) error {

		// return on any error
		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(strings.Replace(file, source, "", -1), string(filepath.Separator))

		if err := writer.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(writer, f); err != nil {
			return err
		}

		return f.Close()
	})
}
