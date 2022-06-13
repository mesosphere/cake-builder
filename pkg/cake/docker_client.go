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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
)

type DockerClient interface {
	Tags(imageName string) (tags []string, err error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options ContainerBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options ContainerPushOptions) (io.ReadCloser, error)
	Close() error
}

type ContainerBuildOptions struct {
	Dockerfile string
	Tags       []string
}

type ContainerPushOptions struct {
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

	err = Tar(config.BaseDir, buildContextTarName)
	if err != nil {
		return err
	}

	dockerBuildContext, err := os.Open(buildContextTarName)
	if err != nil {
		return err
	}
	defer dockerBuildContext.Close()

	log.Printf("Building image with tags: %s", image.getDockerTags(config))

	options := ContainerBuildOptions{
		Dockerfile: image.Dockerfile,
		Tags:       image.getDockerTags(config),
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
	containerPushOptions := ContainerPushOptions{}

	for _, tag := range image.getDockerTags(config) {
		log.Printf("Pushing image with tag: %s", tag)
		out, err := dockerClient.ImagePush(context.Background(), tag, containerPushOptions)

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

func base64Auth(username string, password string) (string, error) {
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}
