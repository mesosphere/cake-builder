package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Running in " + currentDir)

	dryRun := flag.Bool("dry-run", false, "Resolves templates and calculates checksums without build  or pushing images")
	releaseTag := flag.String("release-tag", "latest", "Additional tag to republish checksum based images with e.g. a release tag")
	outputFile := flag.String("out", currentDir+"/cake-report.json", "A file to save build report to")
	registryUrl := flag.String("registry", "https://index.docker.io", "Docker registry URL")
	dockerUser := flag.String("username", "", "Username to authenticate with Docker registry")
	dockerPassword := flag.String("password", "", "Password to authenticate with Docker registry")
	flag.Parse()

	var config BuildConfig
	err = config.loadConfigFromFile(currentDir + "/cake.yaml")
	if err != nil {
		log.Fatal(err)
	}

	config.BaseDir = currentDir
	config.ReleaseTag = *releaseTag
	config.OutputFile = *outputFile

	authConfig := AuthConfig{
		DockerRegistryUrl: *registryUrl,
		Username:          *dockerUser,
		Password:          *dockerPassword,
	}

	config.AuthConfig = authConfig

	log.Println(config.Images)
	log.Println(fmt.Sprintf("[build] dry run: %t, release tag: %s, output file: %s", *dryRun, *releaseTag, *outputFile))

	images, err := transformConfigToImages(config)
	if err != nil {
		log.Fatal(err)
	}

	buildGraph, err := createImageBuildGraph(images)
	if err != nil {
		log.Fatal(err)
	}

	walkBuildGraph(buildGraph, func(image *Image) {
		err = image.renderDockerfileFromTemplate(config)
		if err != nil {
			log.Fatal(err)
		}
		err = image.calculateChecksum()
		if err != nil {
			log.Fatal(err)
		}
	})

	dockerClient := NewExternalDockerClient(config.AuthConfig)

	if !*dryRun {
		walkBuildGraph(buildGraph, func(image *Image) {
			exists, err := imageExists(dockerClient, image, config)
			if err != nil {
				log.Fatal(err)
			}

			if !exists {
				err = buildImage(dockerClient, image, config)
				if err != nil {
					log.Fatal(err)
				}

				err = pushImage(dockerClient, image, config)
				if err != nil {
					log.Fatal(err)
				}
			}
		})
		err = generateReport(buildGraph, config)
		if err != nil {
			log.Fatal(err)
		}
	}
}
