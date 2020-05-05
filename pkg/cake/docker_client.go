package cake

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/jhoonb/archivex"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/heroku/docker-registry-client/registry"
)

type DockerClient interface {
	Tags(imageName string) (tags []string, err error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
}

type ExternalDockerClient struct {
	AuthConfig AuthConfig
	Client     *client.Client
	Registry   *registry.Registry
	TagsCache  map[string][]string
}

// Tags retrieves tags list from the registry for specified image name and adds them to the cache.
// It returns tags list (whether by making an HTTP call to a registry of from the cache) and any error encountered.
func (client *ExternalDockerClient) Tags(imageName string) (tags []string, err error) {
	// check, whether image tags are already in cache for provided image
	tagsCached, inCache := client.TagsCache[imageName]
	if inCache {
		log.Printf("Tags cache hit for image '%s'", imageName)
		return tagsCached, nil
	}

	imageTags, err := client.Registry.Tags(imageName)
	if err == nil {
		// add received tags to the cache
		log.Printf("Caching received tags for '%s' image", imageName)
		client.TagsCache[imageName] = imageTags
	}
	return imageTags, err
}

func (client *ExternalDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	return client.Client.ImageBuild(context.Background(), buildContext, options)
}

func (client *ExternalDockerClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	return client.Client.ImagePush(context.Background(), image, options)
}

func NewExternalDockerClient(authConfig AuthConfig) *ExternalDockerClient {
	dockerClient := ExternalDockerClient{
		AuthConfig: authConfig,
		TagsCache:  make(map[string][]string),
	}

	dockerRegistry, err := registry.New(authConfig.DockerRegistryUrl, authConfig.Username, authConfig.Password)
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		log.Fatal(err)
	}

	dockerClient.Client = cli
	dockerClient.Registry = dockerRegistry
	return &dockerClient
}

func ImageExists(dockerClient DockerClient, image *Image, config BuildConfig) (bool, error) {

	tags, err := dockerClient.Tags(image.getFullName())
	if err != nil {
		return false, fmt.Errorf("unable to retrieve tags for %s: %v", image.getFullName(), err)
	}

	for _, tag := range tags {
		if tag == image.getStableTag(config) {
			log.Printf("Found Image %s with tag: %s", image.getFullName(), image.getStableTag(config))
			return true, nil
		}
	}

	return false, nil
}

func BuildImage(dockerClient DockerClient, image *Image, config BuildConfig) error {
	imageConfig := image.ImageConfig

	tmpDir, err := ioutil.TempDir(os.TempDir(), "cake-build-")
	if err != nil {
		return err
	}

	buildContextTarName := fmt.Sprintf("%s/%s_%s_context.tar", tmpDir, imageConfig.Repository, imageConfig.Name)

	tar := new(archivex.TarFile)
	defer tar.Close()

	err = tar.Create(buildContextTarName)
	if err != nil {
		return err
	}
	err = tar.AddAll(config.BaseDir, false)
	if err != nil {
		return err
	}

	dockerBuildContext, err := os.Open(buildContextTarName)
	if err != nil {
		return err
	}
	defer dockerBuildContext.Close()

	log.Printf("Building image with tags: %s", image.getDockerTags(config))
	base64Auth, err := base64Auth(config)
	if err != nil {
		return err
	}

	authConfigs := make(map[string]types.AuthConfig)
	authConfigs[config.AuthConfig.DockerRegistryUrl] = types.AuthConfig{
		Auth: base64Auth,
	}

	options := types.ImageBuildOptions{
		Dockerfile:  image.Dockerfile,
		Tags:        image.getDockerTags(config),
		AuthConfigs: authConfigs,
	}

	response, err := dockerClient.ImageBuild(context.Background(), dockerBuildContext, options)
	if err != nil {
		return err
	}

	err = handleOutput(response.Body)
	if err != nil {
		return err
	}

	return nil
}

func PushImage(dockerClient DockerClient, image *Image, config BuildConfig) error {
	base64Auth, err := base64Auth(config)
	if err != nil {
		return err
	}

	pushOptions := types.ImagePushOptions{
		RegistryAuth: base64Auth,
	}

	for _, tag := range image.getDockerTags(config) {
		log.Printf("Pushing image with tag: %s", tag)
		out, err := dockerClient.ImagePush(context.Background(), tag, pushOptions)

		if err != nil {
			return fmt.Errorf("error while pushing image %s: %v", tag, err)
		}

		err = handleOutput(out)
		if err != nil {
			return err
		}
	}

	return nil
}

func handleOutput(reader io.ReadCloser) error {
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	err := jsonmessage.DisplayJSONMessagesStream(reader, os.Stderr, termFd, isTerm, func(message jsonmessage.JSONMessage) {
		log.Println(string(*message.Aux))
	})
	if err != nil {
		return fmt.Errorf("error response from Docker daemon: %v", err)
	}
	defer reader.Close()

	return nil
}

func base64Auth(config BuildConfig) (string, error) {
	authConfig := types.AuthConfig{
		Username: config.AuthConfig.Username,
		Password: config.AuthConfig.Password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
