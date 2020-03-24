package cake

import (
	"errors"
	"fmt"
	"sort"
)

// Tree Node represents Docker Image
type Image struct {
	ImageConfig ImageConfig
	Dockerfile  string
	Checksum    string
	Parent      *Image
	Children    []*Image
}

func (image Image) String() string {
	parent := ""
	if image.Parent != nil {
		parentConfig := image.Parent.ImageConfig
		parent = fmt.Sprintf("{Id: %s, Repository: %s, Name: %s}", parentConfig.Id, parentConfig.Repository, parentConfig.Name)
	}

	return fmt.Sprintf("Image{Dockerfile: %s, Checksum: %s, Parent: %s, %s}",
		image.Dockerfile, image.Checksum, parent, image.ImageConfig)
}

func (image Image) getFullName() string {
	return fmt.Sprintf("%s/%s", image.ImageConfig.Repository, image.ImageConfig.Name)
}

func (image Image) getDockerTags(config BuildConfig) []string {
	tags := []string{fmt.Sprintf("%s:%s%s", image.getFullName(), "latest", getTagSuffixStr(image))}

	if len(image.Checksum) > 0 {
		tags = append(tags, fmt.Sprintf("%s:%s%s", image.getFullName(), image.Checksum, getTagSuffixStr(image)))
	}

	if len(config.ReleaseTag) > 0 && "latest" != config.ReleaseTag {
		tags = append(tags, fmt.Sprintf("%s:%s%s", image.getFullName(), config.ReleaseTag, getTagSuffixStr(image)))
	}

	return tags
}

func (image Image) getStableTag(config BuildConfig) string {
	if len(config.ReleaseTag) > 0 && config.ReleaseTag != "latest" {
		return fmt.Sprintf("%s%s", config.ReleaseTag, getTagSuffixStr(image))
	} else if len(image.Checksum) > 0 {
		return fmt.Sprintf("%s%s", image.Checksum, getTagSuffixStr(image))
	} else {
		return fmt.Sprintf("%s%s", "latest", getTagSuffixStr(image))
	}
}

func (image Image) getChecksumTag(config BuildConfig) string {
	return fmt.Sprintf("%s%s", image.Checksum, getTagSuffixStr(image))
}

func getTagSuffixStr(image Image) string {
	if len(image.ImageConfig.TagSuffix) > 0 {
		return fmt.Sprintf("-%s", image.ImageConfig.TagSuffix)
	} else {
		return ""
	}
}

// Transforms list of config items into independent Tree nodes.
// Checks for duplicate IDs and multiple parents (images with no parent defined)
func TransformConfigToImages(config BuildConfig) (images map[string]*Image, err error) {
	imageMap := make(map[string]*Image)
	var baseImage *Image
	for _, imageConfig := range config.Images {
		if _, exists := imageMap[imageConfig.Id]; exists {
			return nil, errors.New("Duplicate Image ID in config: " + imageConfig.Id)
		}

		image := Image{
			ImageConfig: imageConfig,
		}

		if len(imageConfig.Parent) == 0 {
			if baseImage != nil {
				return nil, errors.New(fmt.Sprintf("Multiple base images without declared parents: %s and %s", baseImage.ImageConfig.Id, image.ImageConfig.Id))
			}
			baseImage = &image
		}

		imageMap[imageConfig.Id] = &image
	}

	return imageMap, nil
}

// Constructs a tree/DAG of images and performs cycle detection check and orphaned images check
func CreateImageBuildGraph(images map[string]*Image) (image *Image, err error) {
	// using sorted slice of image ids to maintain consistent building of the target graph
	// which can not be achieved by iterating over the map due to random iteration order
	var ids []string
	for key := range images {
		ids = append(ids, key)
	}
	sort.Strings(ids)

	var root *Image
	for _, key := range ids {
		image := images[key]
		if len(image.ImageConfig.Parent) == 0 {
			root = image
			continue
		}

		imageParent, found := images[image.ImageConfig.Parent]
		if !found || imageParent == nil {
			return nil, errors.New(fmt.Sprintf("Unable to find parent with ID: %s", image.ImageConfig.Parent))
		}

		image.Parent = imageParent
		imageParent.Children = append(imageParent.Children, image)
	}

	if root == nil {
		return nil, errors.New("unable to find base image, check config for cycles")
	}
	//checking for cycles
	visited := make(map[*Image]bool)
	queue := []*Image{root}

	for {
		if len(queue) == 0 {
			break
		}

		image := queue[len(queue)-1]
		if visited[image] {
			return nil, errors.New(fmt.Sprintf("Build hierarchy defined in the config has a cycle, aborting. Image ID: %s", image.ImageConfig.Id))
		} else {
			visited[image] = true
		}

		queue = append(image.Children, queue[:len(queue)-1]...)
	}

	//checking for orphaned images
	for _, key := range ids {
		image := images[key]
		if !visited[image] {
			return nil, errors.New(fmt.Sprintf("Detected orphaned image defined in the config but not found in the build graph. Image ID: %s", image.ImageConfig.Id))
		}
	}

	return root, nil
}

func WalkBuildGraph(graph *Image, apply func(image *Image)) {
	queue := []*Image{graph}
	for {
		if len(queue) == 0 {
			break
		}
		image := queue[len(queue)-1]
		apply(image)
		queue = append(image.Children, queue[:len(queue)-1]...)
	}
}
