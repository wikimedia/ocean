# Ocean

Don't use this yet.

Ocean is a command line tool that uses [Blubber](https://wikitech.wikimedia.org/wiki/Blubber) to create [Docker](https://www.docker.com/) images for building frontend assets. The goal is to use the same Docker images from [docker-registry.wikimedia.org](https://tools.wmflabs.org/dockerregistry/) that will be utilized in the deployment pipeline. This will ensure a consistent environment for development and deployment.

## Installation

Install [Go](https://golang.org) and [Docker](https://www.docker.com/) if you haven't already. Ensure `$GOPATH/bin` is in your `$PATH` by adding something along the lines of `export PATH=$PATH:$GOPATH/bin` to your `.zshrc` or `.bashrc`.

```
go get github.com/wikimedia/ocean
```

## Usage

```
ocean [variant]
```

Running `ocean` will run the `dev` variant by default. This utilizes the `dev` variant from `.pipeline/blubber.yaml` to create a Docker image that runs your service's dev environment. The image will be run in a container with a volume for your working directory so that changes you make on your local machine are read by the service.

`ocean build` is meant to be a stop-gap while built assets are committed. It utilizes the `build` variant from `.pipeline/blubber.yaml` to build the frontend assets in a docker container. The built files will be output to your local filesystem. To clear a `merge` or `cherry-pick` conflict in built assets, perform the appropriate git command, clear any non-built-source conflicts, run `ocean build`, then commit the result.
