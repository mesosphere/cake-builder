package cake

import (
	"context"
	"io"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/heroku/docker-registry-client/registry"
)

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

func containerBuildOptionsToDockerBuildOptions(authConfig AuthConfig, options ContainerBuildOptions) (types.ImageBuildOptions, error) {
	authConfigs := make(map[string]types.AuthConfig)

	base64Auth, err := base64Auth(authConfig.Username, authConfig.Password)
	if err != nil {
		return types.ImageBuildOptions{}, err
	}

	authConfigs[authConfig.DockerRegistryUrl] = types.AuthConfig{Auth: base64Auth}

	return types.ImageBuildOptions{
		Dockerfile:  options.Dockerfile,
		Tags:        options.Tags,
		AuthConfigs: authConfigs,
	}, nil
}

func containerPushOptionsToDockerPushOptions(authConfig AuthConfig, options ContainerPushOptions) (types.ImagePushOptions, error) {
	base64Auth, err := base64Auth(authConfig.Username, authConfig.Password)
	if err != nil {
		return types.ImagePushOptions{}, err
	}

	return types.ImagePushOptions{
		RegistryAuth: base64Auth,
	}, nil
}

func (client *ExternalDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options ContainerBuildOptions) (types.ImageBuildResponse, error) {
	imageBuilderOptions, err := containerBuildOptionsToDockerBuildOptions(client.AuthConfig, options)
	if err != nil {
		return types.ImageBuildResponse{}, err
	}

	return client.Client.ImageBuild(context.Background(), buildContext, imageBuilderOptions)
}

func (client *ExternalDockerClient) ImagePush(ctx context.Context, image string, options ContainerPushOptions) (io.ReadCloser, error) {
	pushOptions, error := containerPushOptionsToDockerPushOptions(client.AuthConfig, options)
	if error != nil {
		return nil, error
	}

	return client.Client.ImagePush(context.Background(), image, pushOptions)
}

func (client *ExternalDockerClient) Close() error {
	return client.Client.Close()
}

func NewExternalDockerClient(authConfig AuthConfig) DockerClient {
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
