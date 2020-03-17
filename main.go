package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

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

	cmd := "build"
	if len(args) > 0 {
		cmd = args[0]
	}

	pkgJSON, err := getPackageJSON("./package.json")
	if err != nil {
		log.Printf("Failed to read package.json %v\n", err)
		os.Exit(6)
	}

	if pkgJSON.Name == "" {
		log.Printf("package.json is missing a name field")
		os.Exit(6)
	}

	dockerfile, err := getDockerfileFromBlubber("./.pipeline/blubber.yaml", cmd)
	if err != nil {
		log.Printf("Failed to create Dockerfile from Blubber config %v\n", err)
		os.Exit(6)
	}

	tag := pkgJSON.Name + "-" + cmd

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

	wd := strings.TrimSpace(string(wdb))
	runCmd := exec.Command("docker", "run", "-v", string(wd)+":/srv/service/", tag)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	err = runCmd.Run()
	if err != nil {
		log.Fatal(err)
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

func getDockerfileFromBlubber(blubberCfgPath string, variant string) (dockerfileBuffer *bytes.Buffer, err error) {
	blubberCfg, err := config.ReadConfigFile(blubberCfgPath)
	if err != nil {
		return
	}

	dockerfileBuffer, err = docker.Compile(blubberCfg, variant)
	return
}
