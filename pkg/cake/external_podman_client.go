package cake

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strings"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/pkg/auth"
	containersTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/domain/entities"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/docker/docker/api/types"
)

type ExternalPodmanClient struct {
	AuthfilePath string
	AuthConfig   AuthConfig
	RegistryHost string
	Ctx          *context.Context
	TagsCache    map[string][]string
}

// Tags retrieves tags list from the registry for specified image name and adds them to the cache.
// It returns tags list (whether by making an HTTP call to a registry of from the cache) and any error encountered.
func (client *ExternalPodmanClient) Tags(imageName string) (imageTags []string, err error) {
	// check, whether image tags are already in cache for provided image

	tagsCached, inCache := client.TagsCache[imageName]
	if inCache {
		log.Printf("Tags cache hit for image '%s'", imageName)
		return tagsCached, nil
	}

	limit := 999
	t := true
	fullName := fmt.Sprintf("%s/%s", client.RegistryHost, imageName)
	results, err := images.Search(*client.Ctx, fullName, &images.SearchOptions{
		Authfile: &client.AuthfilePath,
		Limit:    &limit,
		ListTags: &t,
	})
	if err != nil {
		return nil, err
	}

	imageTags = make([]string, len(results))
	for idx, result := range results {
		imageTags[idx] = result.Tag
	}
	client.TagsCache[imageName] = imageTags
	return imageTags, nil
}

func (client *ExternalPodmanClient) ImageBuild(ctx context.Context, buildContext io.Reader, options ContainerBuildOptions) (types.ImageBuildResponse, error) {
	containerFiles := []string{options.Dockerfile}

	podmanOptions := entities.BuildOptions{
		BuildOptions: buildahDefine.BuildOptions{
			AdditionalTags: options.Tags,
			SystemContext: &containersTypes.SystemContext{
				AuthFilePath: client.AuthfilePath,
			},
		},
	}

	report, err := images.Build(*client.Ctx, containerFiles, podmanOptions)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("report: %v\n", report)
	reader := strings.NewReader(fmt.Sprintf("build image %s", report.ID))
	return types.ImageBuildResponse{Body: io.NopCloser(reader)}, nil
}

func (client *ExternalPodmanClient) Close() error {
	return nil
}

func (client *ExternalPodmanClient) ImagePush(ctx context.Context, image string, options ContainerPushOptions) (io.ReadCloser, error) {
	return nil, images.Push(*client.Ctx, image, image, &images.PushOptions{
		Authfile: &client.AuthfilePath,
	})
}

func NewExternalPodmanClient(authConfig AuthConfig) DockerClient {
	registryUrl, error := url.Parse(authConfig.DockerRegistryUrl)
	if error != nil {
		log.Fatal(error)
	}

	tmpFile, err := ioutil.TempFile("", "auth.json.")

	_, err = tmpFile.Write([]byte{'{', '}'})
	if err != nil {
		log.Fatal(err)
	}
	err = tmpFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	authFilePath := tmpFile.Name()
	podmanClient := ExternalPodmanClient{
		AuthfilePath: authFilePath,
		AuthConfig:   authConfig,
		RegistryHost: registryUrl.Host,
		TagsCache:    make(map[string][]string),
	}

	// Now login to a) test the credentials and to b) store them in
	// the authfile for later use.
	sys := containersTypes.SystemContext{
		AuthFilePath: authFilePath,
	}
	loginOptions := auth.LoginOptions{
		Username: authConfig.Username,
		Password: authConfig.Password,
		AuthFile: authFilePath,
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
	}

	// Get Podman socket location
	sock_dir := os.Getenv("XDG_RUNTIME_DIR")
	if sock_dir == "" {
		sock_dir = "/var/run"
	}
	socket := "unix:" + sock_dir + "/podman/podman.sock"

	// Connect to Podman socket
	ctx, err := bindings.NewConnection(context.Background(), socket)

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Logging in to %s\n", authConfig.DockerRegistryUrl)
	podmanClient.Ctx = &ctx
	err = auth.Login(ctx, &sys, &loginOptions, []string{authConfig.DockerRegistryUrl})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Log in successful\n")

	return &podmanClient
}
