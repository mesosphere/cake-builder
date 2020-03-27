package cake

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRenderDockerfileTemplate(t *testing.T) {
	expectedEnvVar := "test"
	expectedVersionVar := "x.y.z"
	expectedParent := "foo/bar:baz"

	template := `FROM {{parent}}
ENV PROPERTY {{tmpl_property}}
RUN echo "{{tmpl_version}}" > /version.txt
`
	expectedDockerfile := fmt.Sprintf(`FROM %s
ENV PROPERTY %s
RUN echo "%s" > /version.txt
`, expectedParent, expectedEnvVar, expectedVersionVar)

	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	tmpFile, err := os.Create(path.Join(tmpDir, "Dockerfile.template"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = tmpFile.Write([]byte(template))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	image := Image{
		Parent: &Image{
			ImageConfig: ImageConfig{
				Repository: "foo",
				Name:       "bar",
			},
			Checksum: "baz",
		},
		ImageConfig: ImageConfig{
			Template: tmpFile.Name(),
			Properties: map[string]string{
				"tmpl_property": expectedEnvVar,
				"tmpl_version":  expectedVersionVar,
			},
		},
	}

	err = image.RenderDockerfileFromTemplate(BuildConfig{})
	if err != nil {
		t.Errorf("Unexpected error while rendering Dockerfile from template: %v", err)
	}
	dockerfile := path.Join(tmpDir, GeneratedDockerFileNamePrefix+image.ImageConfig.TagSuffix)

	_, err = os.Stat(dockerfile)
	if os.IsNotExist(err) {
		t.Error("Rendered Dockerfile not found")
	}

	bytes, err := ioutil.ReadFile(dockerfile)
	if err != nil {
		t.Errorf("Failed to read bytes from file %s: %v", dockerfile, err)
	}
	contents := string(bytes)

	if strings.Compare(expectedDockerfile, contents) != 0 {
		t.Errorf("Rendered Dockerfile contents differ from the expected.\nExpected:\n%s\nRendered:\n%s", expectedDockerfile, contents)
	}
}

func TestGlobalTemplateProperties(t *testing.T) {
	expectedGlobalProperty := "global_property"
	expectedLocalProperty := "local_property"
	expectedGlobalPropertyOverride := "global_property_override"
	expectedParent := "foo/bar:baz"

	template := `FROM {{parent}}
ENV GLOBAL_PROPERTY {{global_tmpl_property}}
ENV LOCAL_PROPERTY {{local_tmpl_property}}
ENV GLOBAL_PROPERTY_OVERRIDE {{tmpl_property_override}}
`
	expectedDockerfile := fmt.Sprintf(`FROM %s
ENV GLOBAL_PROPERTY %s
ENV LOCAL_PROPERTY %s
ENV GLOBAL_PROPERTY_OVERRIDE %s
`, expectedParent, expectedGlobalProperty, expectedLocalProperty, expectedGlobalPropertyOverride)

	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	tmpFile, err := os.Create(path.Join(tmpDir, "Dockerfile.template"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = tmpFile.Write([]byte(template))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	buildConfig := BuildConfig{
		GlobalProperties: map[string]string{
			"global_tmpl_property":   expectedGlobalProperty,
			"tmpl_property_override": "DEFAULT",
		},
	}

	image := Image{
		Parent: &Image{
			ImageConfig: ImageConfig{
				Repository: "foo",
				Name:       "bar",
			},
			Checksum: "baz",
		},
		ImageConfig: ImageConfig{
			Template: tmpFile.Name(),
			Properties: map[string]string{
				"local_tmpl_property":    expectedLocalProperty,
				"tmpl_property_override": expectedGlobalPropertyOverride,
			},
		},
	}

	err = image.RenderDockerfileFromTemplate(buildConfig)
	if err != nil {
		t.Errorf("Unexpected error while rendering Dockerfile from template: %v", err)
	}
	dockerfile := path.Join(tmpDir, GeneratedDockerFileNamePrefix+image.ImageConfig.TagSuffix)

	_, err = os.Stat(dockerfile)
	if os.IsNotExist(err) {
		t.Error("Rendered Dockerfile not found")
	}

	bytes, err := ioutil.ReadFile(dockerfile)
	if err != nil {
		t.Errorf("Failed to read bytes from file %s: %v", dockerfile, err)
	}
	contents := string(bytes)

	if strings.Compare(expectedDockerfile, contents) != 0 {
		t.Errorf("Rendered Dockerfile contents differ from the expected.\nExpected:\n%s\nRendered:\n%s", expectedDockerfile, contents)
	}
}

func TestErrorOnRenderingMissingTemplateProperties(t *testing.T) {
	template := `FROM {{parent}}
ENV PROPERTY {{property}}
`
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	tmpFile, err := os.Create(path.Join(tmpDir, "Dockerfile.template"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = tmpFile.Write([]byte(template))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	image := Image{
		Parent: &Image{
			ImageConfig: ImageConfig{
				Repository: "foo",
				Name:       "bar",
			},
			Checksum: "baz",
		},
		ImageConfig: ImageConfig{
			Template: tmpFile.Name(),
		},
	}

	err = image.RenderDockerfileFromTemplate(BuildConfig{})
	if err == nil {
		t.Errorf("Expected error while rendering Dockerfile from template with missing variables")
	}
}

func TestListFiles(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}

	tmpFileRoot, err := os.Create(path.Join(tmpDir, "file1"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	levelOneDir := path.Join(tmpDir, "nested")
	err = os.Mkdir(levelOneDir, os.FileMode(0777))
	if err != nil {
		t.Errorf("Failed to create directory %s: %v", levelOneDir, err)
	}
	tmpFileLevelOne, err := os.Create(path.Join(levelOneDir, "file2"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	levelTwoDir := path.Join(tmpDir, "nested2")
	err = os.Mkdir(levelTwoDir, os.FileMode(0777))
	if err != nil {
		t.Errorf("Failed to create directory %s: %v", levelTwoDir, err)
	}
	tmpFileLevelTwo, err := os.Create(path.Join(levelTwoDir, "file3"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	expected := []string{
		tmpFileRoot.Name(),
		tmpFileLevelOne.Name(),
		tmpFileLevelTwo.Name(),
	}
	sort.Strings(expected)

	files, err := listFiles(tmpDir)
	if err != nil {
		t.Errorf("Unexpected error while listing files: %v", err)
	}
	sort.Strings(files)

	if !reflect.DeepEqual(expected, files) {
		t.Errorf("Listed files differ from the expected.\nExpected:\n%s\nFound:\n%s", expected, files)
	}
}

func TestContentChecksum(t *testing.T) {
	testContents := `FROM ubuntu
COMMAND echo "Hello world"
`
	expectedChecksum := checksum(testContents)

	tmpFile, err := ioutil.TempFile("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary file: %v", err)
	}
	_, err = tmpFile.Write([]byte(testContents))
	if err != nil {
		t.Errorf("Failed to write data to temporary file %s: %v", tmpFile.Name(), err)
	}

	calculatedChecksum, err := getContentChecksum(tmpFile.Name())
	if err != nil {
		t.Errorf("Unexpected error while calculating checksum: %v", err)
	}
	if expectedChecksum != calculatedChecksum {
		t.Errorf("Calculated file checksum differs from the expected.\nExpected:\n%s\nCalculated:\n%s", expectedChecksum, calculatedChecksum)
	}
}

func TestCalculateChecksum(t *testing.T) {
	dockerfileContents := `FROM ubuntu
COMMAND echo "Hello world"
`
	sharedFileContents := `#!/usr/bin/env bash
set -e -u

echo "Hello world"
`

	nestedFileContents := `log4j.rootCategory=WARN, console
log4j.appender.console=org.apache.log4j.ConsoleAppender
`
	/*
	   Creating the following folder structure with non-empty files:
	   root/
	       __base/
	           - nested/
	               - nested.file
	           - Dockerfile.generated
	       shared/
	           - shared.file
	*/

	root, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}

	baseDir, err := ioutil.TempDir(root, "__base")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}

	rootDockerfile, err := os.Create(path.Join(baseDir, GeneratedDockerFileNamePrefix))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	_, err = rootDockerfile.Write([]byte(dockerfileContents))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	sharedDir, err := ioutil.TempDir(root, "shared")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}

	sharedFile, err := os.Create(path.Join(sharedDir, "shared.file"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	_, err = sharedFile.Write([]byte(sharedFileContents))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	nestedDir := path.Join(baseDir, "nested")
	err = os.Mkdir(nestedDir, os.FileMode(0777))
	if err != nil {
		t.Errorf("Failed to create directory: %v", err)
	}
	nestedFile, err := os.Create(path.Join(nestedDir, "nested.file"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = nestedFile.Write([]byte(nestedFileContents))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	expectedChecksum := checksum(checksum(dockerfileContents) + checksum(nestedFileContents) + checksum(sharedFileContents))

	image := Image{
		Dockerfile: rootDockerfile.Name(),
		ImageConfig: ImageConfig{
			ExtraFiles: []string{sharedDir},
		},
	}

	err = image.CalculateChecksum()
	if err != nil {
		t.Errorf("Unexpected error while calculating checksum: %v", err)
	}

	if expectedChecksum != image.Checksum {
		t.Errorf("Calculated image checksum differs from the expected.\nExpected:\n%s\nCalculated:\n%s", expectedChecksum, image.Checksum)
	}
}

func TestCalculateChecksumWithMultipleDockerfiles(t *testing.T) {
	primaryDockerfileContents := `FROM ubuntu:18.04
COMMAND echo "Hello world"
`
	secondaryDockerfileContents := `FROM ubuntu:18.10
COMMAND echo "Hello world"
`

	/*
	   Creating the following folder structure with non-empty files:
	   root/
	       image/
	           - Dockerfile.generated
	           - Dockerfile.generated.secondary
	*/

	root, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	baseDir, err := ioutil.TempDir(root, "image")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	primaryDockerfile, err := os.Create(path.Join(baseDir, GeneratedDockerFileNamePrefix))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = primaryDockerfile.Write([]byte(primaryDockerfileContents))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	secondaryDockerfile, err := os.Create(path.Join(baseDir, GeneratedDockerFileNamePrefix+".secondary"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = secondaryDockerfile.Write([]byte(secondaryDockerfileContents))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	expectedChecksum := checksum(checksum(secondaryDockerfileContents))

	image := Image{
		Dockerfile: secondaryDockerfile.Name(),
		ImageConfig: ImageConfig{
			TagSuffix: "secondary",
		},
	}

	err = image.CalculateChecksum()
	if err != nil {
		t.Errorf("Unexpected error while calculating checksum: %v", err)
	}

	if expectedChecksum != image.Checksum {
		t.Errorf("Calculated image checksum differs from the expected.\nExpected:\n%s\nCalculated:\n%s", expectedChecksum, image.Checksum)
	}
}

func checksum(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	return hex.EncodeToString(hash.Sum(nil))
}
