#!/bin/bash
set -e
platforms=("linux/amd64" "darwin/amd64")

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name="cake-${GOOS}-${GOARCH}"
    if [[ ${GOOS} = "windows" ]]; then
        output_name+='.exe'
    fi
    echo "Building for ${platform}: ${output_name}"
    GOOS=${GOOS} GOARCH=${GOARCH} go build -o "${output_name}"
done
