package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type ImageConfig struct {
	Id         string
	Parent     string
	Repository string
	Name       string
	TagSuffix  string `yaml:"tag_suffix"`
	Template   string
	ExtraFiles []string `yaml:"extra_files"`
	Properties map[string]string
}

func (image ImageConfig) String() string {
	out, err := json.Marshal(image)
	if err != nil {
		log.Fatalf("Failed to marshall image config to JSON: %v", err)
	}
	return fmt.Sprintf("ImageConfig%s", out)
}

type AuthConfig struct {
	DockerRegistryUrl string
	Username          string
	Password          string
}

type BuildConfig struct {
	AuthConfig AuthConfig
	BaseDir    string
	ReleaseTag string
	OutputFile string
	Images     []ImageConfig `yaml:"build"`
}

func (config BuildConfig) validate() error {
	for _, image := range config.Images {
		if len(image.Id) == 0 || len(image.Repository) == 0 || len(image.Name) == 0 || len(image.Template) == 0 {
			return fmt.Errorf("Image ID, repository, name, or template is not specified for the following definition: " + image.String())
		}
	}
	return nil
}

func (config *BuildConfig) loadConfigFromFile(fileName string) error {
	configFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal([]byte(configFile), &config)
	if err != nil {
		return fmt.Errorf("cannot unmarshal data: %v", err)
	}

	return nil
}
