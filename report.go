package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type BuildReport struct {
	Images []ImageBuildSummary
}

type ImageBuildSummary struct {
	Id            string
	StableTag     string
	PublishedTags []string
}

func generateReport(image *Image, config BuildConfig) error {
	var summaries []ImageBuildSummary

	walkBuildGraph(image, func(image *Image) {
		summary := ImageBuildSummary{
			Id:            image.ImageConfig.Id,
			StableTag:     fmt.Sprintf("%s:%s", image.getFullName(), image.getStableTag(config)),
			PublishedTags: image.getDockerTags(config),
		}

		summaries = append(summaries, summary)
	})

	report := BuildReport{
		Images: summaries,
	}

	reportJson, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshall build report to JSON: %v", err)
	}

	err = ioutil.WriteFile(config.OutputFile, reportJson, 0644)
	if err != nil {
		return fmt.Errorf("failed to save build report file: %v", err)
	}

	return nil
}
