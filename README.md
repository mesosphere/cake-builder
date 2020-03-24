# Cake Docker Builder

Cake Docker Builder is a tool for managing builds of Docker images organized in hierarchies (layers) in order to
optimize build times, avoid code duplication, enforce deterministic builds, and support reuse of already defined
Dockerfiles.

## Overview
#### Synopsis
Sometimes a project requires producing multiple Docker images which can have different roles and contain different set
of libraries and configuration files specific for their role. Examples include:
- CPU and GPU flavours of the same image
- Different combinations of software versions in the target image (e.g. Spark and Hadoop)
- Images with specific libraries which share common set of dependencies but not used together

Usually, the necessity of producing multiple images is addressed by having an independent `Dockerfile` per image. This
approach, however, has a few shortcomings:
- code duplication between monolithic `Dockerfiles` which have similar build steps increases maintenance burden and
requires the same change to be introduced in every `Dockerfile` in the project
- a need for automated tooling required to build all the images
- lack of tracking changes in source `Dockerfiles` to identify which images require rebuilds
- significant image build times when all the images are rebuilt every time
- disk and network usage which can be an issue for local development

One of the existing approaches to resolving some of these problems is decomposition of the monolithic `Dockerfiles` into
multiple images dependent on each other. This allows to minimize code duplication and reuse already published images to
introduce changes on top of them.

#### Organizing Dockerfiles in a hierarchy (layers)
Images can be organized in a hierarchy where base images contain the shared set of dependencies, and child images use
them to add some specific software on top. Base images are designed in a way to avoid frequent rebuilds and child images
contain the most frequently changing parts like config scripts. Decomposing multiple images in this way allows to
avoid expensive rebuilding of all the images defined as monolithic `Dockerfiles` and target only those which require
changes.

Here's an example project which contains 2 basic kinds of images: `notebook` and `worker`, and each of them has two
flavors: CPU and GPU. The images are organized in a hierarchy to push the most frequently changing ones to the bottom and
avoid rebuilding of the base ones when there is no change.
```
                           base
             (apt dependencies and binaries)
                            |
                            |
                        base-worker
                   (common Conda deps)
                    /                \
                   /                  \
         base-notebook               worker-cpu
      (jupyter Conda deps)       (config and scripts)
               |                          |
               |                          |
         notebook-cpu                 worker-gpu
    (config and scripts)        (CUDA drivers and libs)
               |
               |
          notebook-gpu
    (CUDA drivers and libs)
```

This approach improves the situation with multiple monolithic `Dockerfiles`, however, it still requires specific tooling
to address the following problems:
- identifying images which require a rebuild on change
- rebuilding dependent images when their parent changed
- declarative build parameterization

#### Declarative build parameterization
Docker allows specifying arguments (`ARG` in `Dockerfiles`) and passing them at the build time via
`--build-arg <arg_name>=<arg_value>` flags to `docker` CLI. This approach provides means for parameterizing image
builds. However does not provide a way track the lineage of the published image to the original build command
launched by a build server or from a developer machine. Consider this example:
```dockerfile
FROM continuumio/anaconda:2019.10
ARG TENSORFLOW_VERSION=1.15

RUN conda install tensorflow=${TENSORFLOW_VERSION}
```

When this `Dockerfile` is build without build arguments passed, it will result in an image with TensorFlow version `1.15`
installed. Building the image with the command `docker build -t <target_tag> --build-arg TENSORFLOW_VERSION=2.1.0` will
result in an image with TensorFlow version `2.1.0`. This parametrization is an expected behaviour and a powerful mechanism.
However this flexibility comes with a cost - the `Dockerfile` alone is no longer a complete description of the contents
of images built from it. Without *also* keeping a record of build arguments which were used to build a published image,
often only a forensic-type examination can identify the actual contents.

#### Cake Docker Builder
Cake Docker Builder targets projects which produce multiple Docker images, which in turn have some hierarchy or
dependency between images.

Cake Builder features aim at resolving problems outlined above and include:
- Declarative definition of project layout via config file to explicitly specify dependencies between images
- `Dockerfile` parameterization via templating to provide declarative build parameterization
- Using `Dockerfile` content checksums to identify changes and trigger rebuilds
- Explicit specification of files included in the image and used in checksums to fully represent image contents (e.g.
to avoid rebuilds when README changes)
- Tagging and publishing of images
- Generating build reports in machine-friendly format
- Performing pre-run configuration analysis to avoid cycles, duplicates, and orphaned images

Cake build process has the following main steps:
- Reading configuration from a file named `cake.yaml` from the base directory and create an internal representation of
the image hierarchy
- Replacing templated variables in `Dockerfile.template` and generating `Dockerfiles` used for building images
- Calculating checksums for generated `Dockerfiles` and all additional files included in the image
- Checking DockerHub to identify which images exist to skip rebuilding (using content checksum as a tag)
- Building images using checksums as tags
- Pushing images to DockerHub
- Generating build report

## Building and running
To build the binary, run the following command from the project root:
```
go build -i -o cake ./cmd/cake/main.go
```

It will produce a binary compiled for the current OS/platform. 

Use `build.sh` to build for linux/amd64, darwin/amd64, and windows. 

```
./build.sh
```
This command will create a `dist` directory with all runnable binaries for each platform.

To get a list of available options run:
```
./cake --help
```

There's an example project located in [example](example) folder. To build it one needs to change `repository` in all 
images to an existing repo you have write access to (e.g. you can create a temporary repo `cake-example` in your
DockerHub) and run:
```
cd example
../cake --username=<your DockerHub username> --password="<your DockerHub password>"
```
Any modification of `Dockerfile.template` files, template properties or files listed in `extra_files` should lead to 
a rebuild of affected images. Changes in parent should trigger rebuild of all children.

## Project setup
### Directory layout

Consider an example directory layout for a minimalistic Cake Build project:
```
.
├── base
│   └── Dockerfile.template
├── child
│   └── Dockerfile.template
├── child2
│   ├── Dockerfile.template
│   └── start.sh
├── shared
│   └── shared.sh
└── cake.yaml
```

Cake Builder considers the directory with `cake.yaml` as a project root. This directory also serves as a Docker context
during the build. The organization of Dockerfile templates in folders should be relative to project root and it is
recommended to create a directory per image to keep additional image-specific resources local to the template.

Sometimes there's a set of resources (scripts, configuration files etc.) which is shared between multiple images. In order
to avoid maintenance of multiple copies of the same files in different images, they can be placed in a common folder (`shared`
in this example) and be referenced in a Dockerfile template using relative path from the project root. For example, for
image `child2` which has its own local script and requires a shared script, the Dockerfile template will
look as follows:
```
FROM {{parent}}

COPY child2/start.sh /bin/start.sh
COPY shared/shared.sh /bin/shared.sh
```

Check the [example](example) folder for a sample project layout.

### Configuration
The minimal image definition must contain the following properties:

* `id` - a unique identifier of the image in this build
* `repository` - Docker registry repository
* `name` - target name of the image in Docker repository
* `template` - the location of `Dockerfile.template` file

Additional properties:

* `tag_suffix` - a string appended to image tags during the build using `-` as a separator, e.g. `<image tag>-<tag_suffix>`
* `parent` - ID of another image in this build which is used as a parent of an image in `FROM` statement
* `extra_files` - list of additional files and folders to be included in checksum (files and folders from the same directory where
an image-specific `Dockerfile.template` is located are included by default)
* `properties` - map of properties used for mustache templating to replace variables in `Dockerfile.template`

Example:
```
build: 
  - id: base-image
    repository: akirillov
    name: cake-example
    template: base/Dockerfile.template
    properties:
      ubuntu_version: 18.04
      test: test

  - id: child-image
    parent: base-image
    repository: akirillov
    name: cake-example
    tag_suffix: child_image_tag_suffix
    template: child/Dockerfile.template
    extra_files:
      - shared
    properties:
      spark_version: 2.4.0
```

### Image tag format and publishing
Every image defined in `cake.yaml` results in two tags published to DockerHub which have the following format:
```
<repository>/<name>:<release tag>-<tag_suffix>
<repository>/<name>:<content checksum>-<tag_suffix>
```

**Note:** `<release tag>` in the image tag can be provided via `--release-tag` flag to `cake` CLI and defaults to `latest`.
Every image is published with both the release tag and checksum tag. The checksum tag is used for checking if the image
requires a rebuild based on content checksum while release tag is used additionally for human-readable tags.

The above configuration will result in building images with the following tags:
```
# id: base-image
akirillov/cake-example:latest
akirillov/cake-example:<content checksum>

# id: child-image
akirillov/cake-example:latest-child_image_tag_suffix
akirillov/cake-example:<content checksum>
```

## Limitations
- target repositories must exist in Docker registry (to avoid unwanted auto-creation)
- Multiple hierarchies or multiple single images defined in the config file are not supported by design. Enforcing a
single parent approach leads to a single tree representation which is easier to reason about. As a
workaround/best-practice, one can introduce a simple base image which contains only `FROM x/y:z` step and derive the
rest of the images from it
