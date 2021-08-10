package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

const (
	Version       = "v1.0.0"
	VersionNumber = 100

	defaultImage    = "goredis/grte:latest"
	textEnvFile     = "grte.yaml"
	containerName   = "grte"
	containerGOPATH = "/go"
)

var ctx = context.Background()

type Env struct {
	Image            string            `yaml:"Image"`
	MinVersionNumber int               `yaml:"MinVersionNumber"`
	ContainerEnv     map[string]string `yaml:"ContainerEnv"`

	Cmd     []string
	WorkDir string
	RootDir string
	IsTry   bool
}

var env Env

func init() {
	flag.Parse()
}

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
	}

	// read ~/.grte.yaml
	home, err := os.UserHomeDir()
	if err == nil && fileIsExist(filepath.Join(home, "."+textEnvFile)) {
		homeEnvBuff, err := os.ReadFile(filepath.Join(home, "."+textEnvFile))
		if err != nil {
			return err
		}
		if err = yaml.Unmarshal(homeEnvBuff, &env); err != nil {
			return err
		}
	}
	if VersionNumber < env.MinVersionNumber {
		return errors.New("the tool version is too low, please upgrade the version")
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
			return
		case "help", "h":
			logf("Usage: %s [OPTION | COMMAND]", cmd)
			logf("Option:")
			logf("    -h --help\tPrint help and quit")
			logf("    -v --version\tPrint version information and quit")
			logf("COMMAND: command to be execute")
			logf("    %s go test ./...", cmd)
			logf("    %s golangci-lint run", cmd)
			return
		default:
			os.Args = append(os.Args[1:], strings.Split(os.Args[1], " ")...)
		}
	}

	if err := before(); err != nil {
		iconLogln(errorIcon, err)
		return
	}

	if err := exec(); err != nil {
		iconLogln(errorIcon, err)
		return
	}

	iconLogln(successIcon, "Success!")
}

func exec() error {
	cli, err := GetDockerClient(ctx)
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
	iconLogln(dockerIcon, "Create docker container...")

	containerEnv := make([]string, 0, len(env.ContainerEnv))
	for k, v := range env.ContainerEnv {
		containerEnv = append(containerEnv, fmt.Sprintf("%s=%s", k, v))
	}
	config := &container.Config{
		Image:      env.Image,
		Env:        containerEnv,
		WorkingDir: env.WorkDir,
		Tty:        env.IsTry,
	}

	localCacheGoPath := filepath.Join(os.TempDir(), "go-redis-test-env-gopath")
	if !fileIsExist(localCacheGoPath) {
		_ = os.Mkdir(localCacheGoPath, 0777)
	}
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: env.RootDir,
			Target: env.RootDir,
		},
		{
			Type:   mount.TypeBind,
			Source: localCacheGoPath,
			Target: containerGOPATH,
		},
	}
	hostConfig := &container.HostConfig{
		Privileged:  true,
		Mounts:      mounts,
		NetworkMode: "host",
	}

	create, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return err
	}
	defer removeContainer(cli)

	for _, warn := range create.Warnings {
		iconLogln(dockerIcon, warn)
	}
	if err := cli.ContainerStart(ctx, create.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	iconLogf(dockerIcon, "ContainerID: %s", create.ID)
	iconLogf(dockerIcon, "WorkDir: %s", env.WorkDir)
	iconLogf(dockerIcon, "Command: %s", env.Cmd)

	cmd := fmt.Sprintf("cp -r /redis %s && %s && rm -rf %s",
		filepath.Join(env.RootDir, "testdata"),
		strings.Join(env.Cmd, " "),
		filepath.Join(env.RootDir, "testdata/redis"),
	)
	idResp, err := cli.ContainerExecCreate(ctx, create.ID, types.ExecConfig{
		User:         "grte",
		Cmd:          []string{"sh", "-c", cmd},
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

// GetDockerClient get the docker client, if it is not installed, return an error.
func GetDockerClient(ctx context.Context) (*client.Client, error) {
	var err error
	var cli *client.Client

	dockerHost := os.Getenv("DOCKER_HOST")
	if strings.HasPrefix(dockerHost, "ssh://") {
		var helper *connhelper.ConnectionHelper

		helper, err = connhelper.GetConnectionHelper(dockerHost)
		if err != nil {
			return nil, err
		}
		cli, err = client.NewClientWithOpts(
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	} else {
		cli, err = client.NewClientWithOpts(client.FromEnv)
	}
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(ctx)

	return cli, err
}

// ---------------------------------------

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

// ------------------------------------

const (
	noneIcon    = ""
	errorIcon   = " \u274C  "
	successIcon = " \u2705  "
	dockerIcon  = " \U0001F433 "
)

type logger struct {
	out io.Writer
}

func (l *logger) Print(v ...interface{})   { _, _ = l.out.Write([]byte(fmt.Sprint(v...))) }
func (l *logger) Println(v ...interface{}) { _, _ = l.out.Write([]byte(fmt.Sprintln(v...))) }
func (l *logger) Printf(format string, v ...interface{}) {
	_, _ = l.out.Write([]byte(fmt.Sprintf(format, v...)))
}

var log = logger{out: os.Stdout}

// dockerMessages is the json data output by docker,
// we only take the required fields and omit part of it.
type dockerMessage struct {
	Stream         string      `json:"stream"`
	Status         string      `json:"status"`
	Progress       string      `json:"progress"`
	ProgressDetail interface{} `json:"progressDetail"`
	ID             string      `json:"id"`
	Error          string      `json:"error"`
	ErrorDetail    struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func readDockerOutput(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	defer body.Close()

	scanner := bufio.NewScanner(body)

	var (
		msg     dockerMessage
		lastErr error
	)
	posMap := make(map[string]int)
	next := func() {
		for k := range posMap {
			posMap[k]++
		}
	}
	for scanner.Scan() {
		line := scanner.Bytes()

		msg.ID = ""
		msg.Stream = ""
		msg.Error = ""
		msg.ErrorDetail.Message = ""
		msg.Status = ""
		msg.Progress = ""
		msg.ProgressDetail = nil

		if err := json.Unmarshal(line, &msg); err != nil {
			iconLogf(errorIcon, "unable to unmarshal line [%s] ==> %v", string(line), err)
			lastErr = err
			next()
			continue
		}
		if msg.Error != "" {
			iconLogln(errorIcon, msg.Error)
			lastErr = errors.New(msg.Error)
			break
		}

		if msg.ErrorDetail.Message != "" {
			iconLogln(errorIcon, msg.ErrorDetail.Message)
			lastErr = errors.New(msg.Error)
			break
		}
		if msg.Status != "" {
			msg.Status = strings.TrimSpace(msg.Status)
			if msg.Progress != "" {
				dockerLogProgress(posMap[msg.ID], "%s :: %s :: %s", msg.Status, msg.ID, msg.Progress)
			} else if msg.ProgressDetail != nil && msg.ID != "" {
				if idx, ok := posMap[msg.ID]; ok {
					dockerLogProgress(idx, "%s :: %s", msg.Status, msg.ID)
				} else {
					iconLogf(dockerIcon, "%s :: %s", msg.Status, msg.ID)
					posMap[msg.ID] = 0
					next()
				}
			} else if msg.ID != "" {
				iconLogf(dockerIcon, "%s :: %s", msg.Status, msg.ID)
				next()
			} else {
				iconLogln(dockerIcon, msg.Status)
				next()
			}
		} else if msg.Stream != "" {
			iconLogln(dockerIcon, msg.Stream)
			next()
		} else {
			lastErr = fmt.Errorf("unable to handle line: %s", string(line))
			iconLogln(errorIcon, lastErr)
			next()
		}
	}
	return lastErr
}

func logf(format string, args ...interface{}) {
	log.Println(logFormat(noneIcon, format, args...))
}

func iconLogln(icon string, v ...interface{}) {
	log.Println(logFormat(icon, fmt.Sprint(v...)))
}

func iconLogf(icon, format string, args ...interface{}) {
	log.Println(logFormat(icon, format, args...))
}

func iconFail(icon string, v ...interface{}) {
	log.Println(logFormat(icon, fmt.Sprint(v...)))
	os.Exit(1)
}

func logProgress(pos int, format string, args ...interface{}) {
	log.Printf("\033[%dA\r\033[K%s\033[%dB\r", pos, logFormat(noneIcon, format, args...), pos)
}

func dockerLogProgress(pos int, format string, args ...interface{}) {
	log.Printf("\033[%dA\r\033[K%s\033[%dB\r", pos, logFormat(dockerIcon, format, args...), pos)
}

func logFormat(icon, format string, args ...interface{}) string {
	var s string
	if icon != noneIcon {
		s = fmt.Sprintf(fmt.Sprintf("%s%s", icon, format), args...)
	} else {
		s = fmt.Sprintf(format, args...)
	}

	return fmt.Sprintf("\x1b[%dm[%s] \x1b[0m%s", 36, "go-redis", s)
}
