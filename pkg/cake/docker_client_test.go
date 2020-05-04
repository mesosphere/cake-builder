package cake

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
)

type MockDockerClient struct {
	MockTagsResponse       []string
	MockImageBuildResponse types.ImageBuildResponse
	ImageBuildOptions      types.ImageBuildOptions
	ImagePushOptions       types.ImagePushOptions
	ImagePushTags          []string
}

func (client *MockDockerClient) Tags(imageName string) (tags []string, err error) {
	return client.MockTagsResponse, nil
}

func (client *MockDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	client.ImageBuildOptions = options
	response := types.ImageBuildResponse{
		Body: ioutil.NopCloser(strings.NewReader(`{"message": "image built"}`)),
	}
	//setting input options here to verify them in test
	client.ImageBuildOptions = options
	return response, nil
}

func (client *MockDockerClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	client.ImagePushOptions = options
	//this method called multiple times so we need to collect all the pushed tags
	client.ImagePushTags = append(client.ImagePushTags, image)
	return ioutil.NopCloser(strings.NewReader(`{"message": "image pushed"}`)), nil
}

func TestImageExists(t *testing.T) {
	testTag := "test_tag"
	client := new(MockDockerClient)
	client.MockTagsResponse = []string{testTag}

	image := Image{ImageConfig: ImageConfig{Repository: "repository", Name: "image-name"}}
	config := BuildConfig{
		ReleaseTag: testTag,
	}

	exists, err := ImageExists(client, &image, config)
	if err != nil {
		t.Error(err)
	}

	if !exists {
		t.Errorf("Expected to find the tag but 'imageExists' returned false")
	}

	config.ReleaseTag = "nonexistent"
	exists, err = ImageExists(client, &image, config)
	if err != nil {
		t.Error(err)
	}

	if exists {
		t.Errorf("Expected the tag to be absent but 'imageExists' returned true")
	}
}

func TestImageBuild(t *testing.T) {
	baseDir, err := ioutil.TempDir("", "base") //Docker build context root
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	_, err = os.Create(path.Join(baseDir, "script.sh"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	buildConfig := BuildConfig{
		BaseDir:    baseDir,
		ReleaseTag: "1.0",
		AuthConfig: AuthConfig{
			DockerRegistryUrl: "https://index.docker.io",
			Username:          "user",
			Password:          "password",
		},
	}

	imageConfig := ImageConfig{
		Repository: "repository",
		Name:       "image-name",
	}

	image := Image{
		Dockerfile:  "base/Dockerfile",
		ImageConfig: imageConfig,
		Checksum:    "12w21ew",
	}

	expectedTags := []string{
		"repository/image-name:latest",
		"repository/image-name:" + buildConfig.ReleaseTag,
		"repository/image-name:" + image.Checksum,
	}
	sort.Strings(expectedTags)

	dockerClient := new(MockDockerClient)
	err = BuildImage(dockerClient, &image, buildConfig)
	if err != nil {
		t.Error(err)
	}

	buildOptions := dockerClient.ImageBuildOptions

	if buildOptions.Dockerfile != image.Dockerfile {
		t.Errorf("Expected Dockerfile %s but found %s in ImageBuildOptions", image.Dockerfile, buildOptions.Dockerfile)
	}

	sort.Strings(buildOptions.Tags)
	if !reflect.DeepEqual(expectedTags, buildOptions.Tags) {
		t.Errorf("Tags in ImageBuildOptions differ from the expected.\nExpected:\n%s\nFound:\n%s", expectedTags, buildOptions.Tags)
	}

	if len(buildOptions.AuthConfigs) != 1 {
		t.Errorf("Expected single entry in AuthConfigs but found: %s", buildOptions.AuthConfigs)
	}

	authConfig, found := buildOptions.AuthConfigs[buildConfig.AuthConfig.DockerRegistryUrl]

	if !found {
		t.Errorf("Expected config is not present in AuthConfigs for registry: %s", buildConfig.AuthConfig.DockerRegistryUrl)
	}

	base64Auth, err := base64Auth(buildConfig)
	if err != nil {
		t.Error(err)
	}

	if authConfig.Auth != base64Auth {
		t.Errorf("Base64 Auth differs from expected in ImageBuildOptions.AuthConfigs:\nExpected:\n%s\nFound:\n%s", base64Auth, authConfig.Auth)
	}
}

func TestPushImage(t *testing.T) {
	baseDir, err := ioutil.TempDir("", "base") //Docker build context root
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	_, err = os.Create(path.Join(baseDir, "script.sh"))
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}

	buildConfig := BuildConfig{
		ReleaseTag: "1.0",
		AuthConfig: AuthConfig{
			DockerRegistryUrl: "https://index.docker.io",
			Username:          "user",
			Password:          "password",
		},
	}

	imageConfig := ImageConfig{
		Repository: "repository",
		Name:       "image-name",
	}

	image := Image{
		ImageConfig: imageConfig,
		Checksum:    "12w21ew",
	}

	expectedTags := []string{
		"repository/image-name:latest",
		"repository/image-name:" + buildConfig.ReleaseTag,
		"repository/image-name:" + image.Checksum,
	}
	sort.Strings(expectedTags)

	dockerClient := new(MockDockerClient)
	err = PushImage(dockerClient, &image, buildConfig)
	if err != nil {
		t.Error(err)
	}

	pushOptions := dockerClient.ImagePushOptions

	base64Auth, err := base64Auth(buildConfig)
	if err != nil {
		t.Error(err)
	}
	if pushOptions.RegistryAuth != base64Auth {
		t.Errorf("Base64 Auth differs from expected in ImagePushOptions.RegistryAuth:\nExpected:\n%s\nFound:\n%s", base64Auth, pushOptions.RegistryAuth)
	}

	sort.Strings(dockerClient.ImagePushTags)
	if !reflect.DeepEqual(expectedTags, dockerClient.ImagePushTags) {
		t.Errorf("Tags in ImagePushTags differ from the expected.\nExpected:\n%s\nFound:\n%s", expectedTags, dockerClient.ImagePushTags)
	}
}

func TestBase64Auth(t *testing.T) {
	buildConfig := BuildConfig{
		AuthConfig: AuthConfig{
			Username: "user",
			Password: "password",
		},
	}

	//Base64 for the following JSON: {"username":"user","password":"password"}
	base64 := "eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InBhc3N3b3JkIn0="

	base64Auth, err := base64Auth(buildConfig)
	if err != nil {
		t.Error(err)
	}
	if base64 != base64Auth {
		t.Errorf("Base64-encoded Auth object differs from expected.\nExpected:\n%s\nFound:\n%s", base64, base64Auth)
	}
}
