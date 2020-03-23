package cake

import (
	"fmt"
	"reflect"
	"testing"
)

const org = "organisation"
const repo = "image-name"
const fullName = "organisation/image-name"
const testChecksum = "fde9532"
const releaseTag = "1.0"
const tagSuffix = "suffix"

func TestGetFullName(t *testing.T) {
	image := Image{
		ImageConfig: ImageConfig{
			Repository: org,
			Name:       repo,
		},
	}

	if fullName != image.getFullName() {
		t.Errorf("Expected '%s' but received '%s'", fullName, image.getFullName())
	}
}

func TestGetDockerTags(t *testing.T) {
	expected := []string{
		fmt.Sprintf("%s:%s", fullName, "latest"),
		fmt.Sprintf("%s:%s", fullName, testChecksum),
		fmt.Sprintf("%s:%s", fullName, releaseTag),
	}

	image := Image{
		ImageConfig: ImageConfig{
			Repository: org,
			Name:       repo,
		},
		Checksum: testChecksum,
	}

	tags := image.getDockerTags(BuildConfig{
		ReleaseTag: releaseTag,
	})

	if !reflect.DeepEqual(expected, tags) {
		t.Errorf("Tags differ from the expected.\nExpected:\n%s\nFound:\n%s", expected, tags)
	}

	//Test no release tag
	expected = []string{
		fmt.Sprintf("%s:%s", fullName, "latest"),
		fmt.Sprintf("%s:%s", fullName, testChecksum),
	}

	tags = image.getDockerTags(BuildConfig{})
	if !reflect.DeepEqual(expected, tags) {
		t.Errorf("Tags differ from the expected.\nExpected:\n%s\nFound:\n%s", expected, tags)
	}
}

func TestGetStableTag(t *testing.T) {
	image := Image{
		ImageConfig: ImageConfig{
			TagSuffix: tagSuffix,
		},
	}

	//test release tag
	tag := image.getStableTag(BuildConfig{ReleaseTag: releaseTag})
	expected := fmt.Sprintf("%s-%s", releaseTag, tagSuffix)
	if expected != tag {
		t.Errorf("Expected: %s. Found: %s", expected, tag)
	}

	//test no release tag, no checksum
	tag = image.getStableTag(BuildConfig{})
	expected = fmt.Sprintf("latest-%s", tagSuffix)
	if expected != tag {
		t.Errorf("Expected: %s. Found: %s", expected, tag)
	}

	//test no release tag, checksum present
	image.Checksum = testChecksum
	tag = image.getStableTag(BuildConfig{})
	expected = fmt.Sprintf("%s-%s", testChecksum, tagSuffix)
	if expected != tag {
		t.Errorf("Expected: %s. Found: %s", expected, tag)
	}
}

func TestGetTagSuffix(t *testing.T) {
	image := Image{
		ImageConfig: ImageConfig{
			TagSuffix: "",
		},
	}

	//test no tag suffix
	tSuffix := getTagSuffixStr(image)
	if tSuffix != "" {
		t.Errorf("Expected  no tag suffix but found: %s", tSuffix)
	}

	//test with tag suffix present
	image.ImageConfig.TagSuffix = tagSuffix
	tSuffix = getTagSuffixStr(image)
	expected := fmt.Sprintf("-%s", tagSuffix)
	if tSuffix != expected {
		t.Errorf("Expected tag suffix: '%s' but found: %s", expected, tSuffix)
	}
}

func TestTransformConfigToImages(t *testing.T) {
	/*Testing the following hierarchy:

	          parent
	            |
	          child-0
	            /\
	           /  \
	          /    \
	  child-01      child-02

	*/
	buildConfig := BuildConfig{
		Images: []ImageConfig{
			{
				Id:   "parent",
				Name: "base-image",
			},
			{
				Id:     "child-0",
				Name:   "child-image",
				Parent: "parent",
			},
			{
				Id:     "child-01",
				Name:   "child-image-1",
				Parent: "child-0",
			},
			{
				Id:     "child-02",
				Name:   "child-image-2",
				Parent: "child-0",
			},
		},
	}

	images, err := TransformConfigToImages(buildConfig)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedImages := make(map[string]*Image)
	for _, imageConfig := range buildConfig.Images {
		expectedImages[imageConfig.Id] = &Image{ImageConfig: imageConfig}
	}

	if !reflect.DeepEqual(expectedImages, images) {
		t.Errorf("Images created from config differ from the expected.\nExpected:\n%s\nFound:\n%s", expectedImages, images)
	}
}

func TestMultipleBaseImagesDetection(t *testing.T) {
	/*Testing the following hierarchy:

	  parent-0 	parent-1
	     |
	  child-0

	*/
	buildConfig := BuildConfig{
		Images: []ImageConfig{
			{
				Id:   "parent-0",
				Name: "base-image",
			},
			{
				Id:     "child-0",
				Name:   "child-image",
				Parent: "parent-0",
			},
			{
				Id:   "parent-1",
				Name: "parent-image-1",
			},
		},
	}

	images, err := TransformConfigToImages(buildConfig)
	if err == nil {
		t.Errorf("Expected error but received images: %s", images)
	}

	expectedError := "Multiple base images without declared parents: parent-0 and parent-1"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%v', but received '%v'.", expectedError, err)
	}
}

func TestMissingImageIdDetection(t *testing.T) {
	/*Testing the following hierarchy:

	  image-1
	     |
	  image-1

	*/
	buildConfig := BuildConfig{
		Images: []ImageConfig{
			{
				Id:   "image-1",
				Name: "base-image",
			},
			{
				Id:     "image-1",
				Name:   "child-image",
				Parent: "image-1",
			},
		},
	}

	images, err := TransformConfigToImages(buildConfig)
	if err == nil {
		t.Errorf("Expected error but received images: %s", images)
	}

	expectedError := "Duplicate Image ID in config: image-1"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%v', but received = '%v'.", expectedError, err)
	}
}

func TestCreateBuildGraph(t *testing.T) {
	/*Expecting the following hierarchy:

	          parent
	            |
	          child-0
	            /\
	           /  \
	          /    \
	  child-01      child-02

	*/

	sourceImages := map[string]*Image{
		"parent":   {ImageConfig: ImageConfig{Id: "parent"}},
		"child-0":  {ImageConfig: ImageConfig{Id: "child-0", Parent: "parent"}},
		"child-01": {ImageConfig: ImageConfig{Id: "child-01", Parent: "child-0"}},
		"child-02": {ImageConfig: ImageConfig{Id: "child-02", Parent: "child-0"}},
	}

	parent, err := CreateImageBuildGraph(sourceImages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if parent.ImageConfig.Id != "parent" {
		t.Errorf("Expected parent with id 'parent' but received: %s", parent.ImageConfig.Id)
	}

	if len(parent.Children) != 1 {
		t.Errorf("Expected single child on level one but got %d", len(parent.Children))
	}

	childLevelOne := parent.Children[0]

	if childLevelOne.ImageConfig.Id != "child-0" {
		t.Errorf("Expected level one child with id 'child-0' but received: %s", childLevelOne.ImageConfig.Id)
	}

	if childLevelOne.Parent.ImageConfig.Id != "parent" {
		t.Errorf("Parent ID is incorrect for level one child expected 'parent' but got %s", childLevelOne.Parent.ImageConfig.Id)
	}

	if len(childLevelOne.Children) != 2 {
		t.Errorf("Expected 2 child nodes for level one child but got %s", childLevelOne.Children)
	}

	if childLevelOne.Children[0].Parent.ImageConfig.Id != "child-0" || childLevelOne.Children[1].Parent.ImageConfig.Id != "child-0" {
		t.Errorf("Level two nodes parent is incorrect. Expected 'child-0' but got [for child 1]: '%s', [for child 2]: %s",
			childLevelOne.Children[0].Parent.ImageConfig.Id, childLevelOne.Children[1].Parent.ImageConfig.Id)
	}
}

func TestImageIdNotFound(t *testing.T) {
	sourceImages := map[string]*Image{
		"parent":  {ImageConfig: ImageConfig{Id: "parent"}},
		"child-0": {ImageConfig: ImageConfig{Id: "child-0", Parent: "unknown"}},
	}

	root, err := CreateImageBuildGraph(sourceImages)
	if err == nil {
		t.Errorf("Expected error but received %s", root)
	}

	expectedError := "Unable to find parent with ID: unknown"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%v', but received = '%v'.", expectedError, err)
	}
}

func TestOrphanedImageDetection(t *testing.T) {
	sourceImages := map[string]*Image{
		"root":    {ImageConfig: ImageConfig{Id: "root"}},
		"child-0": {ImageConfig: ImageConfig{Id: "child-0", Parent: "child-1"}},
		"child-1": {ImageConfig: ImageConfig{Id: "child-1", Parent: "child-0"}},
	}

	root, err := CreateImageBuildGraph(sourceImages)
	if err == nil {
		t.Errorf("Expected error but received %s", root)
	}

	expectedError := "Detected orphaned image defined in the config but not found in the build graph. Image ID: child-0"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%v', but received = '%v'.", expectedError, err)
	}
}

func TestNoBaseImage(t *testing.T) {
	sourceImages := map[string]*Image{
		"child-0": {ImageConfig: ImageConfig{Id: "child-0", Parent: "child-1"}},
		"child-1": {ImageConfig: ImageConfig{Id: "child-1", Parent: "child-0"}},
	}

	root, err := CreateImageBuildGraph(sourceImages)
	if err == nil {
		t.Errorf("Expected error but received %s", root)
	}

	expectedError := "unable to find base image, check config for cycles"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%v', but received = '%v'.", expectedError, err)
	}
}

func TestWalkBuildGraph(t *testing.T) {
	sourceImages := map[string]*Image{
		"root":     {ImageConfig: ImageConfig{Id: "root"}},
		"child-0":  {ImageConfig: ImageConfig{Id: "child-0", Parent: "root"}},
		"child-1":  {ImageConfig: ImageConfig{Id: "child-1", Parent: "root"}},
		"child-10": {ImageConfig: ImageConfig{Id: "child-10", Parent: "child-1"}},
		"child-11": {ImageConfig: ImageConfig{Id: "child-11", Parent: "child-1"}},
	}

	expected := []string{"root", "child-1", "child-0", "child-11", "child-10"}

	root, err := CreateImageBuildGraph(sourceImages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	var visited []string

	WalkBuildGraph(root, func(image *Image) {
		visited = append(visited, image.ImageConfig.Id)
	})

	if !reflect.DeepEqual(expected, visited) {
		t.Errorf("Node order in a tree traversal differs from the expected.\nExpected:\n%s\nFound:\n%s", expected, visited)
	}
}
