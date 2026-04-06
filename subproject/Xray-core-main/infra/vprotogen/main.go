package main

import (
	"errors"
	"flag"
	"fmt"
	"go/build"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var directory = flag.String("pwd", "", "Working directory of Xray vprotogen.")

// envFile returns the name of the Go environment configuration file.
// Copy from https://github.com/golang/go/blob/c4f2a9788a7be04daf931ac54382fbe2cb754938/src/cmd/go/internal/cfg/cfg.go#L150-L166
func envFile() (string, error) {
	if file := os.Getenv("GOENV"); file != "" {
		if file == "off" {
			return "", errors.New("GOENV=off")
		}
		return file, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", errors.New("missing user-config dir")
	}
	return filepath.Join(dir, "go", "env"), nil
}

// GetRuntimeEnv returns the value of runtime environment variable,
// that is set by running following command: `go env -w key=value`.
func GetRuntimeEnv(key string) (string, error) {
	file, err := envFile()
	if err != nil {
		return "", err
	}
	if file == "" {
		return "", errors.New("missing runtime env file")
	}
	var data []byte
	var runtimeEnv string
	data, readErr := os.ReadFile(file)
	if readErr != nil {
		return "", readErr
	}
	envStrings := strings.Split(string(data), "\n")
	for _, envItem := range envStrings {
		envItem = strings.TrimSuffix(envItem, "\r")
		envKeyValue := strings.Split(envItem, "=")
		if strings.EqualFold(strings.TrimSpace(envKeyValue[0]), key) {
			runtimeEnv = strings.TrimSpace(envKeyValue[1])
		}
	}
	return runtimeEnv, nil
}

// GetGOBIN returns GOBIN environment variable as a string. It will NOT be empty.
func GetGOBIN() string {
	// The one set by user explicitly by `export GOBIN=/path` or `env GOBIN=/path command`
	GOBIN := os.Getenv("GOBIN")
	if GOBIN == "" {
		var err error
		// The one set by user by running `go env -w GOBIN=/path`
		GOBIN, err = GetRuntimeEnv("GOBIN")
		if err != nil {
			// The default one that Golang uses
			return filepath.Join(build.Default.GOPATH, "bin")
		}
		if GOBIN == "" {
			return filepath.Join(build.Default.GOPATH, "bin")
		}
		return GOBIN
	}
	return GOBIN
}

func whichProtoc(suffix, targetedVersion string) (string, error) {
	protoc := "protoc" + suffix

	path, err := exec.LookPath(protoc)
	if err != nil {
		return "", fmt.Errorf(`
Command "%s" not found.
Make sure that %s is in your system path or current path.
Download %s v%s or later from https://github.com/protocolbuffers/protobuf/releases
`, protoc, protoc, protoc, targetedVersion)
	}
	return path, nil
}

func getProjectProtocVersion(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.New("can not get the version of protobuf used in xray project")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("can not read from body")
	}
	versionRegexp := regexp.MustCompile(`\/\/\s*protoc\s*v\d+\.(\d+\.\d+)`)
	matched := versionRegexp.FindStringSubmatch(string(body))
	return matched[1], nil
}

func getInstalledProtocVersion(protocPath string) (string, error) {
	cmd := exec.Command(protocPath, "--version")
	cmd.Env = append(cmd.Env, os.Environ()...)
	output, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		return "", cmdErr
	}
	versionRegexp := regexp.MustCompile(`protoc\s*(\d+\.\d+)`)
	matched := versionRegexp.FindStringSubmatch(string(output))
	return matched[1], nil
}

func parseVersion(s string, width int) int64 {
	strList := strings.Split(s, ".")
	format := fmt.Sprintf("%%s%%0%ds", width)
	v := ""
	for _, value := range strList {
		v = fmt.Sprintf(format, v, value)
	}
	var result int64
	var err error
	if result, err = strconv.ParseInt(v, 10, 64); err != nil {
		return 0
	}
	return result
}

func needToUpdate(targetedVersion, installedVersion string) bool {
	vt := parseVersion(targetedVersion, 4)
	vi := parseVersion(installedVersion, 4)
	return vt > vi
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of vprotogen:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if !filepath.IsAbs(*directory) {
		pwd, wdErr := os.Getwd()
		if wdErr != nil {
			fmt.Println("Can not get current working directory.")
			os.Exit(1)
		}
		*directory = filepath.Join(pwd, *directory)
	}

	pwd := *directory
	GOBIN := GetGOBIN()
	binPath := os.Getenv("PATH")
	pathSlice := []string{pwd, GOBIN, binPath}
	binPath = strings.Join(pathSlice, string(os.PathListSeparator))
	os.Setenv("PATH", binPath)

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	/*
		targetedVersion, err := getProjectProtocVersion("https://raw.githubusercontent.com/XTLS/Xray-core/HEAD/core/config.pb.go")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	*/
	targetedVersion := ""

	protoc, err := whichProtoc(suffix, targetedVersion)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	installedVersion, err := getInstalledProtocVersion(protoc)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if needToUpdate(targetedVersion, installedVersion) {
		fmt.Printf(`
You are using an old protobuf version, please update to v%s or later.
Download it from https://github.com/protocolbuffers/protobuf/releases

    * Protobuf version used in xray project: v%s
    * Protobuf version you have installed: v%s

`, targetedVersion, targetedVersion, installedVersion)
		os.Exit(1)
	}

	protoFilesMap := make(map[string][]string)
	walkErr := filepath.Walk(pwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if info.IsDir() {
			return nil
		}

		dir := filepath.Dir(path)
		filename := filepath.Base(path)
		if strings.HasSuffix(filename, ".proto") {
			path = path[len(pwd)+1:]
			protoFilesMap[dir] = append(protoFilesMap[dir], path)
		}

		return nil
	})
	if walkErr != nil {
		fmt.Println(walkErr)
		os.Exit(1)
	}

	for _, files := range protoFilesMap {
		for _, relProtoFile := range files {
			args := []string{
				"--go_out", pwd,
				"--go_opt", "paths=source_relative",
				"--go-grpc_out", pwd,
				"--go-grpc_opt", "paths=source_relative",
				"--plugin", "protoc-gen-go=" + filepath.Join(GOBIN, "protoc-gen-go"+suffix),
				"--plugin", "protoc-gen-go-grpc=" + filepath.Join(GOBIN, "protoc-gen-go-grpc"+suffix),
			}
			args = append(args, relProtoFile)
			cmd := exec.Command(protoc, args...)
			cmd.Env = append(cmd.Env, os.Environ()...)
			cmd.Dir = pwd
			output, cmdErr := cmd.CombinedOutput()
			if len(output) > 0 {
				fmt.Println(string(output))
			}
			if cmdErr != nil {
				fmt.Println(cmdErr)
				os.Exit(1)
			}
		}
	}
}
