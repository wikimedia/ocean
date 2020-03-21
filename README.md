# Ocean

**IMPORTANT: Don't use this yet**

Ocean is a command line tool creates standardized frontend JS build and development environments with [Docker](https://www.docker.com/). It uses [Blubber](https://wikitech.wikimedia.org/wiki/Blubber) to build off of the Docker images from [docker-registry.wikimedia.org](https://tools.wmflabs.org/dockerregistry/) that will be used in the deployment pipeline. This will ensure a consistent environment for development and deployment.

## Installation

Install [Go](https://golang.org) and [Docker](https://www.docker.com/) if you haven't already. Ensure `$GOPATH/bin` is in your `$PATH` by adding something along the lines of `export PATH=$PATH:$GOPATH/bin` to your `.zshrc` or `.bashrc`.

```
go get github.com/wikimedia/ocean
```

## Usage

Create `.ocean/config.yml`. This describes how to turn the Blubber `dev` and `build` variants into `docker-compose` configurations for building and development.

```
version: 1.0
variants:
    dev:
        services:
            mobileapps:
                ports:
                    - "8888:8888"
            pagelib:
                path: pagelib
    build:
        services:
            pagelib:
                path: pagelib
```

```
ocean dockerize
```

Running `ocean dockerize` will update the Dockerfiles and docker-compose.yml files in the repository. These files are intended to be committed to the repository so that anyone familiar with docker can `docker-compose up` and have a development enviroment running.

```
ocean [variant]
```

Running `ocean` will run the `dev` variant by default.

`ocean build` is meant to be a stop-gap while built assets are committed. It utilizes the `build` variant from `.pipeline/blubber.yaml` to build the frontend assets in a docker container. The built files will be output to your local filesystem. To clear a `merge` or `cherry-pick` conflict in built assets, perform the appropriate git command, clear any non-built-source conflicts, run `ocean build`, then commit the result.
