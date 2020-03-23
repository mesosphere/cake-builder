#!/bin/bash
set -e
platforms=("linux/amd64" "darwin/amd64" "windows")

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name="cake"
    if [[ ${GOOS} = "windows" ]]; then
        output_name+="-${GOOS}.exe"
    else
        output_name+="-${GOOS}-${GOARCH}"
    fi

    echo "Building for ${platform}: ${output_name}"
    GOOS=${GOOS} GOARCH=${GOARCH} go build -o "${output_name}" ./cmd/cake/main.go
done
