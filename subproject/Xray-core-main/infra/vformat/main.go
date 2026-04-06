package main

import (
	"errors"
	"flag"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var directory = flag.String("pwd", "", "Working directory of Xray vformat.")

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
		if len(envKeyValue) == 2 && strings.TrimSpace(envKeyValue[0]) == key {
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

func Run(binary string, args []string) ([]byte, error) {
	cmd := exec.Command(binary, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	output, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		return nil, cmdErr
	}
	return output, nil
}

func RunMany(binary string, args, files []string) {
	fmt.Println("Processing...")

	maxTasks := make(chan struct{}, runtime.NumCPU())
	for _, file := range files {
		maxTasks <- struct{}{}
		go func(file string) {
			output, err := Run(binary, append(args, file))
			if err != nil {
				fmt.Println(err)
			} else if len(output) > 0 {
				fmt.Println(string(output))
			}
			<-maxTasks
		}(file)
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of vformat:\n")
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
	gofmt := "gofumpt" + suffix
	goimports := "gci" + suffix

	if gofmtPath, err := exec.LookPath(gofmt); err != nil {
		fmt.Println("Can not find", gofmt, "in system path or current working directory.")
		os.Exit(1)
	} else {
		gofmt = gofmtPath
	}

	if goimportsPath, err := exec.LookPath(goimports); err != nil {
		fmt.Println("Can not find", goimports, "in system path or current working directory.")
		os.Exit(1)
	} else {
		goimports = goimportsPath
	}

	rawFilesSlice := make([]string, 0, 1000)
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
		if strings.HasSuffix(filename, ".go") &&
			!strings.HasSuffix(filename, ".pb.go") &&
			!strings.Contains(dir, filepath.Join("testing", "mocks")) &&
			!strings.Contains(path, filepath.Join("main", "distro", "all", "all.go")) {
			rawFilesSlice = append(rawFilesSlice, path)
		}

		return nil
	})
	if walkErr != nil {
		fmt.Println(walkErr)
		os.Exit(1)
	}

	gofmtArgs := []string{
		"-s", "-l", "-e", "-w",
	}

	goimportsArgs := []string{
		"write",
	}

	RunMany(gofmt, gofmtArgs, rawFilesSlice)
	RunMany(goimports, goimportsArgs, rawFilesSlice)
	fmt.Println("Do NOT forget to commit file changes.")
}
