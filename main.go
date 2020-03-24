package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"gerrit.wikimedia.org/r/blubber/config"
	"gerrit.wikimedia.org/r/blubber/docker"
	"github.com/pborman/getopt/v2"
	"gopkg.in/yaml.v2"
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

	oceanConfig, err := getOceanConfig(".ocean/config.yml")
	if err != nil {
		log.Printf("Failed to read ocean config %v\n", err)
	}

	if variant == "dockerize" {
		dockerize(oceanConfig)
	} else {
		run(oceanConfig, variant)
	}
}

func dockerize(oceanConfig Ocean) {
	for variantName, variant := range oceanConfig.Variants {
		dockerCompose := DockerCompose{Version: "3.7"}
		dockerCompose.Services = map[string]DockerComposeService{}
		for serviceName, service := range variant.Services {
			if service.Path == "" {
				service.Path = "."
			} else {
				service.Path = "./" + service.Path
			}
			blubberPath := service.Path + "/.pipeline/blubber.yaml"
			blubberVariantName := service.Blubber["variant"]
			if blubberVariantName == "" {
				blubberVariantName = variantName
			}
			dockerFileName := getDockerFileNameForBlubberVariant(blubberVariantName)
			dockerfilePath := service.Path + "/" + dockerFileName
			dockerfileBuffer, blubberCfg, err := getDockerFileDataFromBlubber(blubberPath, blubberVariantName)
			if err != nil {
				log.Fatal(err)
			}
			dockerfileData, err := ioutil.ReadAll(dockerfileBuffer)
			if err != nil {
				log.Fatal(err)
			}
			err = ioutil.WriteFile(dockerfilePath, dockerfileData, 0600)
			if err != nil {
				log.Fatal(err)
			}
			mainVolume := service.Path + ":" + blubberCfg.Lives.In
			// Exclude /node_modules from the volume so it uses the modules from the
			// container and not the ones from your local filesystem
			nodeModulesExclusion := blubberCfg.Lives.In + "/node_modules"
			volumes := []string{mainVolume, nodeModulesExclusion}
			build := map[string]string{"dockerfile": dockerFileName, "context": service.Path}
			dockerCompose.Services[serviceName+getSuffixForVariant(variantName)] = DockerComposeService{Build: build, Ports: service.Ports, Volumes: volumes, Command: service.Command}
		}
		dockerComposeFileData, err := yaml.Marshal(&dockerCompose)
		if err != nil {
			log.Fatal(err)
		}
		dockerComposePath := getDockerComposeFileNameForVariant(variantName)
		err = ioutil.WriteFile(dockerComposePath, dockerComposeFileData, 0600)
		if err != nil {
			log.Fatal(err)
		}
		if variantName == "dev" {
			// Create a symbolic link so that vanilla `docker-compose up` works for dev
			lnCmd := exec.Command("ln", "-s", dockerComposePath, "docker-compose.yml")
			lnCmd.Run()
		}
	}
}

func run(oceanConfig Ocean, variantName string) {
	dockerComposePath := getDockerComposeFileNameForVariant(variantName)
	if _, err := os.Stat(dockerComposePath); os.IsNotExist(err) {
		dockerize(oceanConfig)
	}
	runDockerCompose(dockerComposePath)
}

func getSuffixForVariant(variantName string) (suffix string) {
	return "-" + variantName
}

func getDockerComposeFileNameForVariant(variantName string) string {
	return "docker-compose" + getSuffixForVariant(variantName) + ".yml"
}

func getDockerFileNameForBlubberVariant(blubberVariantName string) string {
	return "Dockerfile" + getSuffixForVariant(blubberVariantName)
}

func runDockerCompose(dockerComposeFilePath string) {
	// Use syscall.Exec to make docker the current process so ctrl + c stops the containers
	dockerBinary, lookErr := exec.LookPath("docker-compose")
	if lookErr != nil {
		log.Fatal(lookErr)
	}

	dockerArgs := []string{"docker-compose", "-f", dockerComposeFilePath, "up", "--build", "--remove-orphans"}
	dockerEnv := os.Environ()
	execErr := syscall.Exec(dockerBinary, dockerArgs, dockerEnv)
	if execErr != nil {
		panic(execErr)
	}
}

// OceanService representation of service in variant
type OceanService struct {
	Path    string
	Ports   []string
	Command string
	Blubber map[string]string
}

// OceanVariant representation of variant
type OceanVariant struct {
	Services map[string]OceanService
}

// Ocean config
type Ocean struct {
	Version  string
	Variants map[string]OceanVariant
}

func getOceanConfig(path string) (config Ocean, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(data, &config)
	return
}

func getDockerFileDataFromBlubber(blubberCfgPath string, variant string) (dockerfileBuffer *bytes.Buffer, blubberCfg *config.Config, err error) {
	blubberCfg, err = config.ReadConfigFile(blubberCfgPath)
	if err != nil {
		return
	}
	dockerfileBuffer, err = docker.Compile(blubberCfg, variant)
	return
}

// DockerCompose representation
type DockerCompose struct {
	Version  string
	Services map[string]DockerComposeService
}

// DockerComposeService representation
type DockerComposeService struct {
	Image   string            `yaml:"image,omitempty"`
	Build   map[string]string `yaml:"build,omitempty"`
	Ports   []string          `yaml:"ports,omitempty"`
	Volumes []string          `yaml:"volumes,omitempty"`
	Command string            `yaml:"command,omitempty"`
}

func getDockerCompose(path string) (pkg DockerCompose, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(data, &pkg)
	return
}

// Run once by building and creating a temp dockerfile with the image tags
// Not currently used, but is an alternative option if we don't want to commit docker configs
func runOnce(oceanConfig Ocean, variant string) {
	dockerCompose := DockerCompose{Version: "3.7"}
	dockerCompose.Services = map[string]DockerComposeService{}
	oceanVariant := oceanConfig.Variants[variant]
	for serviceName, service := range oceanVariant.Services {
		if service.Path == "" {
			service.Path = "."
		} else {
			service.Path = "./" + service.Path
		}
		blubberPath := service.Path + "/.pipeline/blubber.yaml"
		dockerfileBuffer, blubberCfg, err := getDockerFileDataFromBlubber(blubberPath, variant)
		if err != nil {
			log.Fatal(err)
		}
		tag := serviceName + "-" + variant
		buildCmd := exec.Command("docker", "build", "--tag", tag, "--file", "-", ".")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		buildStdin, err := buildCmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			defer buildStdin.Close()
			dockerfileBuffer.WriteTo(buildStdin)
		}()
		err = buildCmd.Run()
		if err != nil {
			log.Fatal(err)
		}
		mainVolume := service.Path + ":" + blubberCfg.Lives.In
		nodeModulesExclusion := blubberCfg.Lives.In + "/node_modules"
		volumes := []string{mainVolume, nodeModulesExclusion}
		image := tag
		dockerCompose.Services[serviceName] = DockerComposeService{Image: image, Ports: service.Ports, Volumes: volumes}
	}
	dockerComposeFileData, err := yaml.Marshal(&dockerCompose)
	if err != nil {
		log.Fatal(err)
	}
	dockerComposePath := ".temp-docker-compose-" + variant + ".yml"
	err = ioutil.WriteFile(dockerComposePath, dockerComposeFileData, 0600)
	if err != nil {
		log.Fatal(err)
	}
	pwdCmd := exec.Command("pwd")
	wdb, err := pwdCmd.Output()
	wd := strings.TrimSpace(string(wdb))
	defer os.Remove(wd + dockerComposePath)

	runDockerCompose(dockerComposePath)
}
