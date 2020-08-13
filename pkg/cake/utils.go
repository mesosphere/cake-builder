package cake

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/facebookgo/symwalk"

	"github.com/cbroglie/mustache"
)

const GeneratedDockerFileNamePrefix = "Dockerfile.generated"
const DefaultShaLength = 64

func (image *Image) RenderDockerfileFromTemplate(config BuildConfig) error {
	directory := filepath.Dir(image.ImageConfig.Template)

	templateProperties := config.GlobalProperties

	if templateProperties == nil {
		templateProperties = make(map[string]string)
	}

	if len(image.ImageConfig.Properties) == 0 && len(templateProperties) == 0 {
		log.Printf("No properties provided for templating")
	} else {
		for key, value := range image.ImageConfig.Properties {
			templateProperties[key] = value
		}
	}

	if image.Parent != nil {
		templateProperties["parent"] = fmt.Sprintf("%s:%s", image.Parent.getFullName(), image.Parent.getStableTag(config))
	}

	mustache.AllowMissingVariables = false
	rendered, err := mustache.RenderFile(image.ImageConfig.Template, templateProperties)
	if err != nil {
		return fmt.Errorf("error while rendering template: %v", err)
	}

	// including prefix and suffix into the generated file name for better readability
	dockerfile := fmt.Sprintf("%s/%s", directory, GeneratedDockerFileNamePrefix)
	if len(image.ImageConfig.TagPrefix) > 0 {
		dockerfile = fmt.Sprintf("%s.%s", dockerfile, image.ImageConfig.TagPrefix)
	}
	if len(image.ImageConfig.TagSuffix) > 0 {
		dockerfile = fmt.Sprintf("%s.%s", dockerfile, image.ImageConfig.TagSuffix)
	}

	err = ioutil.WriteFile(dockerfile, []byte(rendered), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed while writing templated file: %v", err)
	}

	image.Dockerfile = dockerfile
	return nil
}

func (image *Image) CalculateChecksum(checksumLength int) error {
	directory := filepath.Dir(image.Dockerfile)
	files, err := listFiles(directory)
	if err != nil {
		return fmt.Errorf("error while listing files in directory: %s. %v", directory, err)
	}

	// Filtering out files and folders excluded from checksums in the config.
	// Also, filtering out generated Dockerfiles not belonging to the current image.
	// This is required for the cases when a single Dockerfile.template is used for multiple images
	// with different parameters. Each image defined in cake.yaml using the same Dockerfile.template
	// will generate Dockerfile.generated[<tag suffix>] used in checksum for that specific image.
	filteredFiles := make([]string, 0)
	for _, file := range files {
		isGeneratedDockerfile := strings.Contains(file, GeneratedDockerFileNamePrefix)
		isImageFile := file == image.Dockerfile

		excluded := false
		for _, s := range image.ImageConfig.ExcludedFiles {
			if strings.HasPrefix(file, s) {
				excluded = true
				break
			}
		}

		if (!isGeneratedDockerfile || isImageFile) && !excluded {
			filteredFiles = append(filteredFiles, file)
		}
	}
	files = filteredFiles

	for _, file := range image.ImageConfig.ExtraFiles {
		info, err := os.Stat(file)

		if os.IsNotExist(err) {
			return fmt.Errorf("one of the extra paths specified for checksum doesn't exist: %s", file)
		}

		if info.IsDir() {
			dirFiles, err := listFiles(file)
			if err != nil {
				return fmt.Errorf("error while listing files in directory: %s. %v", directory, err)
			}
			files = append(files, dirFiles...)
		} else {
			files = append(files, file)
		}
	}

	sort.Strings(files)
	imageDetailsStr := fmt.Sprintf("[%s][%s]", image.ImageConfig.TagPrefix, image.ImageConfig.TagSuffix)

	log.Printf("Files used for content checksum for %s%s:", image.ImageConfig.Name, imageDetailsStr)
	for _, file := range files {
		log.Println(file)
	}

	checksums := ""

	for _, file := range files {
		contentChecksum, err := getContentChecksum(file)
		if err != nil {
			return err
		} else {
			checksums = checksums + contentChecksum
		}

	}

	hash := sha256.New()
	hash.Write([]byte(checksums))
	//converting checksum to string and truncating to the specified checksumLength
	checksum := hex.EncodeToString(hash.Sum(nil))[:checksumLength]
	log.Printf("Resulting checksum for %s%s: %s", image.ImageConfig.Name, imageDetailsStr, checksum)
	image.Checksum = checksum
	return nil
}

func getContentChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed calculating checksum: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func listFiles(directory string) ([]string, error) {
	files := make([]string, 0)
	err := symwalk.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && info.Mode() != os.ModeSymlink {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
