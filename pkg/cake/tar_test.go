package cake

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mholt/archiver"
)

func TestTar(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}

	source := "testdata/symlinked/main"
	target := path.Join(tmpDir, "test.tar")

	err = Tar(source, target)
	if err != nil {
		t.Errorf("Failed to create tar archive: %v", err)
	}

	// using standard library to untar
	file, err := os.Open(target)
	if err != nil {
		t.Errorf("Error opening tar file: %v", err)
	}
	defer file.Close()

	err = archiver.Unarchive(target, tmpDir)
	if err != nil {
		t.Errorf("Error reading tar file: %v", err)
	}

	files := make([]string, 0)
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && info.Mode() != os.ModeSymlink {
			files = append(files, path)
		}
		return nil
	})

	sort.Strings(files)

	expected := []string{
		tmpDir + "/Dockerfile.generated",
		tmpDir + "/external/external.txt",
		tmpDir + "/test.tar",
	}

	if !reflect.DeepEqual(expected, files) {
		t.Errorf("Listed files differ from the expected.\nExpected:\n%s\nFound:\n%s", expected, files)
	}
}
