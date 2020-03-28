package cake

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigFromFile(t *testing.T) {
	config := `global_properties:
  global_key_1: global_value_1
  global_key_2: global_value_2

images:
  - id: base
    repository: testorg
    name: test
    template: base/Dockerfile.template
    tag_suffix: base
    properties:
      base_key_1: base_value_1
      base_key_2: base_value_2
    extra_files:
      - base_file_1
      - base_file_2

  - id: child
    parent: base
    repository: testorg
    name: test
    template: child/Dockerfile.template
    tag_suffix: child
    properties:
      child_key_1: child_value_1
      child_key_2: child_value_2
    extra_files:
      - child_file_1
      - child_file_2
`

	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	tmpFile, err := os.Create(path.Join(tmpDir, "cake.yaml"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	_, err = tmpFile.Write([]byte(config))
	if err != nil {
		t.Errorf("Failed to write file: %v", err)
	}

	var buildConfig BuildConfig
	err = buildConfig.LoadConfigFromFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Failed to unmarshall file: %v", err)
	}

	assert.NotNil(t, buildConfig)
	assert.NotNil(t, buildConfig.GlobalProperties)
	assert.Equal(t, 2, len(buildConfig.GlobalProperties))
	assert.Equal(t, "global_value_1", buildConfig.GlobalProperties["global_key_1"])
	assert.Equal(t, "global_value_2", buildConfig.GlobalProperties["global_key_2"])

	assert.NotNil(t, buildConfig.Images)
	assert.Equal(t, 2, len(buildConfig.Images))

	var base ImageConfig
	var child ImageConfig

	for _, image := range buildConfig.Images {
		if image.Id == "base" {
			base = image
		}
		if image.Id == "child" {
			child = image
		}
	}
	assert.NotNil(t, base)
	assert.NotNil(t, child)

	assert.Equal(t, "testorg", base.Repository)
	assert.Equal(t, "test", base.Name)
	assert.Equal(t, "base/Dockerfile.template", base.Template)
	assert.Equal(t, "base", base.TagSuffix)

	assert.Equal(t, 2, len(base.Properties))
	assert.Equal(t, "base_value_1", base.Properties["base_key_1"])
	assert.Equal(t, "base_value_2", base.Properties["base_key_2"])

	assert.Equal(t, 2, len(base.ExtraFiles))
	assert.Contains(t, base.ExtraFiles, "base_file_1")
	assert.Contains(t, base.ExtraFiles, "base_file_2")

	assert.Equal(t, "testorg", child.Repository)
	assert.Equal(t, "test", child.Name)
	assert.Equal(t, "child/Dockerfile.template", child.Template)
	assert.Equal(t, "child", child.TagSuffix)

	assert.Equal(t, 2, len(child.Properties))
	assert.Equal(t, "child_value_1", child.Properties["child_key_1"])
	assert.Equal(t, "child_value_2", child.Properties["child_key_2"])

	assert.Equal(t, 2, len(child.ExtraFiles))
	assert.Contains(t, child.ExtraFiles, "child_file_1")
	assert.Contains(t, child.ExtraFiles, "child_file_2")
}
