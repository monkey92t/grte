package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	actContainer "github.com/nektos/act/pkg/container"
)

const (
	Version       = "v1.0.0"
	VersionNumber = 100

	defaultImage  = "goredis/grte:latest"
	textEnvFile   = "grte.yaml"
	containerName = "go-redis-test"
)

var ctx = context.Background()

type Env struct {
	Image            string `yaml:"Image"`
	MinVersionNumber int    `yaml:"MinVersionNumber"`

	Cmd     []string
	WorkDir string
	RootDir string
	IsTry   bool
}

var env Env

func before() error {
	var err error

	env = Env{}
	env.Cmd = os.Args[1:]
	env.IsTry = term.IsTerminal(int(os.Stdout.Fd()))

	env.WorkDir, _ = os.Getwd()
	env.RootDir, err = goRedisRoot(env.WorkDir)
	if err != nil {
		return err
	}

	if fileIsExist(filepath.Join(env.RootDir, textEnvFile)) {
		envBuff, err := os.ReadFile(filepath.Join(env.RootDir, textEnvFile))
		if err != nil {
			return err
		}
		if err = yaml.Unmarshal(envBuff, &env); err != nil {
			return err
		}
		if VersionNumber < env.MinVersionNumber {
			return errors.New("the tool version is too low, please upgrade the version")
		}
	}

	if env.Image == "" {
		env.Image = defaultImage
	}

	return nil
}

func main() {
	switch cmd := filepath.Base(os.Args[0]); len(os.Args) {
	case 1:
		logf("please enter the test command, such as `%s go test ./...`", cmd)
		return
	case 2:
		switch strings.ToLower(strings.TrimLeft(os.Args[1], "-")) {
		case "version", "v":
			logf("%s -- %s", filepath.Base(os.Args[0]), Version)
		case "help", "h":
			logf("Usage: %s [OPTION | COMMAND]", cmd)
			logf("Option:")
			logf("    -h --help\tPrint help and quit")
			logf("    -v --version\tPrint version information and quit")
			logf("COMMAND: command to be execute")
			logf("    %s go test ./...", cmd)
			logf("    %s golangci-lint run", cmd)
		}
		return
	}

	if err := before(); err != nil {
		iconLogln(errorIcon, err)
	}

	if err := exec(); err != nil {
		iconLogln(errorIcon, err)
	}
}

func exec() error {
	cli, err := actContainer.GetDockerClient(ctx)
	if err != nil {
		return err
	}
	removeContainer(cli)

	iconLogf(dockerIcon, "Prepare docker image ==> %s", env.Image)

	pull := true
	if env.Image != defaultImage {
		inspect, _, err := cli.ImageInspectWithRaw(ctx, env.Image)
		if err == nil && inspect.ID != "" {
			iconLogf(dockerIcon, "Use image cache, ID ==> %s", inspect.ID)
			pull = false
		}
	}
	if pull {
		body, err := cli.ImagePull(ctx, dockerImageRef(env.Image), types.ImagePullOptions{})
		if err != nil {
			return err
		}
		if err := readDockerOutput(body); err != nil {
			return err
		}
	}

	return runContainer(cli)
}

func runContainer(cli *client.Client) error {
	iconLogln(dockerIcon, "Create Docker Container")
	config := &container.Config{
		Image:      env.Image,
		WorkingDir: env.WorkDir,
		Tty:        env.IsTry,
	}
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: env.RootDir,
			Target: env.RootDir,
		},
	}
	hostConfig := &container.HostConfig{
		Mounts:      mounts,
		NetworkMode: "host",
	}

	create, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return err
	}
	for _, warn := range create.Warnings {
		iconLogln(dockerIcon, warn)
	}
	if err := cli.ContainerStart(ctx, create.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	iconLogln(dockerIcon, create.ID)

	defer removeContainer(cli)

	iconLogf(dockerIcon, "exec command %s", env.Cmd)
	idResp, err := cli.ContainerExecCreate(ctx, create.ID, types.ExecConfig{
		Cmd:          env.Cmd,
		WorkingDir:   env.WorkDir,
		Tty:          env.IsTry,
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		return err
	}

	resp, err := cli.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{
		Tty: env.IsTry,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	if !env.IsTry || os.Getenv("NORAW") != "" {
		_, err = stdcopy.StdCopy(os.Stdout, os.Stdout, resp.Reader)
	} else {
		_, err = io.Copy(os.Stdout, resp.Reader)
	}
	if err != nil {
		return err
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, idResp.ID)
	if err != nil {
		return err
	}

	if inspectResp.ExitCode == 0 {
		return nil
	}

	return fmt.Errorf("exit with `FAILURE`: %v", inspectResp.ExitCode)
}

var rootErr = errors.New("need to execute commands in the go-redis directory")

func goRedisRoot(pwd string) (string, error) {
	for pwd != "/" {
		if fileIsExist(filepath.Join(pwd, textEnvFile)) || fileIsExist(filepath.Join(pwd, ".github")) {
			return pwd, nil
		}
		pwd = filepath.Dir(pwd)
	}
	return "", rootErr
}

func fileIsExist(f string) bool {
	_, err := os.Stat(f)
	if err == nil || os.IsExist(err) {
		return true
	}
	return false
}

func dockerImageRef(s string) string {
	if s == "" || strings.HasPrefix(s, "docker.io/") {
		return s
	}
	return fmt.Sprintf("docker.io/%s", strings.TrimPrefix(s, "/"))
}

func removeContainer(cli *client.Client) {
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		iconFail(errorIcon, err)
	}

removed:
	for _, c := range containers {
		for _, name := range c.Names {
			if name[1:] == containerName {
				err = cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{Force: true})
				if err != nil {
					iconFail(errorIcon, err)
				}
				break removed
			}
		}
	}
}
