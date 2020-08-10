package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mesosphere/cake-builder/pkg/cake"
)

func main() {

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Running in " + currentDir)

	dryRun := flag.Bool("dry-run", false, "Resolves templates and calculates checksums without building or pushing images")
	releaseTag := flag.String("release-tag", "latest", "Additional tag to republish checksum based images with e.g. a release tag")
	outputFile := flag.String("out", currentDir+"/cake-report.json", "A file to save build report to")
	registryUrl := flag.String("registry", "https://index.docker.io", "Docker registry URL")
	dockerUser := flag.String("username", "", "Username to authenticate with Docker registry")
	dockerPassword := flag.String("password", "", "Password to authenticate with Docker registry")
	checksumLength := flag.Int("checksum-length", cake.DefaultShaLength, fmt.Sprintf("Truncate the resulting checksum tag to the specified length within the interval [1, %d]", cake.DefaultShaLength))
	flag.Parse()

	var config cake.BuildConfig
	err = config.LoadConfigFromFile(currentDir + "/cake.yaml")
	if err != nil {
		log.Fatal(err)
	}

	if *checksumLength <= 0 || *checksumLength > 64 {
		log.Fatalf("Invalid checksum length value. Expected value should be in the interval [1, %d] but was %d.", cake.DefaultShaLength, *checksumLength)
	}

	config.BaseDir = currentDir
	config.ReleaseTag = *releaseTag
	config.OutputFile = *outputFile
	config.AuthConfig = cake.AuthConfig{
		DockerRegistryUrl: *registryUrl,
		Username:          *dockerUser,
		Password:          *dockerPassword,
	}

	log.Println(config.Images)
	log.Println(fmt.Sprintf("[build] dry run: %t, release tag: %s, output file: %s", *dryRun, *releaseTag, *outputFile))

	images, err := cake.TransformConfigToImages(config)
	if err != nil {
		log.Fatal(err)
	}

	buildGraph, err := cake.CreateImageBuildGraph(images)
	if err != nil {
		log.Fatal(err)
	}

	cake.WalkBuildGraph(buildGraph, func(image *cake.Image) {
		err = image.RenderDockerfileFromTemplate(config)
		if err != nil {
			log.Fatal(err)
		}
		err = image.CalculateChecksum(*checksumLength)
		if err != nil {
			log.Fatal(err)
		}
	})

	if !*dryRun {
		dockerClient := cake.NewExternalDockerClient(config.AuthConfig)
		defer dockerClient.Client.Close()

		cake.WalkBuildGraphParallel(buildGraph, func(image *cake.Image) {
			exists, err := cake.ImageExists(dockerClient, image, config)
			if err != nil {
				log.Fatal(err)
			}

			if !exists {
				err = cake.BuildImage(dockerClient, image, config)
				if err != nil {
					log.Fatal(err)
				}

				err = cake.PushImage(dockerClient, image, config)
				if err != nil {
					log.Fatal(err)
				}
			}
		})
		err = cake.GenerateReport(buildGraph, config)
		if err != nil {
			log.Fatal(err)
		}
	}
}
