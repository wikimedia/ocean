package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"gerrit.wikimedia.org/r/blubber/config"
	"gerrit.wikimedia.org/r/blubber/docker"
	"github.com/pborman/getopt/v2"
)

const parameters = "command"

var (
	showHelp = getopt.BoolLong("help", 'h', "show help/usage")
)

func main() {
	getopt.SetParameters(parameters)
	getopt.Parse()

	if *showHelp {
		getopt.Usage()
		os.Exit(1)
	}

	args := getopt.Args()

	variant := "dev"
	if len(args) > 0 {
		variant = args[0]
	}

	oceanJSON, err := getOceanJSON(".ocean/config.json")
	oceanVariant := oceanJSON.Variants[variant]

	pkgJSON, err := getPackageJSON("./package.json")
	if err != nil {
		log.Printf("Failed to read package.json %v\n", err)
		os.Exit(6)
	}

	if pkgJSON.Name == "" {
		log.Printf("package.json is missing a name field")
		os.Exit(6)
	}

	dockerfile, err := getDockerfileFromBlubber("./.pipeline/blubber.yaml", variant)
	if err != nil {
		log.Printf("Failed to create Dockerfile from Blubber config %v\n", err)
		os.Exit(6)
	}

	tag := pkgJSON.Name + "-" + variant

	buildCmd := exec.Command("docker", "build", "--tag", tag, "--file", "-", ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildStdin, err := buildCmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer buildStdin.Close()
		dockerfile.WriteTo(buildStdin)
	}()

	err = buildCmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	pwdCmd := exec.Command("pwd")
	wdb, err := pwdCmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	// Switch to syscall.Exec to make docker the current process so ctrl + c stops the container
	wd := strings.TrimSpace(string(wdb))
	dockerBinary, lookErr := exec.LookPath("docker")
	if lookErr != nil {
		panic(lookErr)
	}
	// --volume /srv/service/node_modules excludes node_modules from the volume so that the versions installed in the container are used
	dockerArgs := []string{"docker", "run", "--rm", "--interactive", "--tty", "--volume", string(wd) + ":/srv/service/", "--volume", "/srv/service/node_modules"}
	if oceanVariant.Port != 0 {
		port := strconv.FormatInt(oceanVariant.Port, 10)
		dockerArgs = append(dockerArgs, "-p", port+":"+port)
	}
	dockerArgs = append(dockerArgs, tag)
	dockerEnv := os.Environ()
	execErr := syscall.Exec(dockerBinary, dockerArgs, dockerEnv)
	if execErr != nil {
		panic(execErr)
	}
}

// PackageJSON representation of node.js package.json
type PackageJSON struct {
	Name string
}

func getPackageJSON(path string) (pkg PackageJSON, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &pkg)
	return
}

// Ocean representation of scheme in .ocean/config.json
type OceanVariant struct {
	Port int64 // ports aren't 64 bit but it makes this easier to convert to a string
}

// Ocean representation of .ocean/config.json
type Ocean struct {
	Version  string
	Variants map[string]OceanVariant
}

func getOceanJSON(path string) (pkg Ocean, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &pkg)
	return
}

func getDockerfileFromBlubber(blubberCfgPath string, variant string) (dockerfileBuffer *bytes.Buffer, err error) {
	blubberCfg, err := config.ReadConfigFile(blubberCfgPath)
	if err != nil {
		return
	}

	dockerfileBuffer, err = docker.Compile(blubberCfg, variant)
	return
}
