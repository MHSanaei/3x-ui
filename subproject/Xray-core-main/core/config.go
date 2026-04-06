package core

import (
	"io"
	"slices"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/cmdarg"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/main/confloader"
	"google.golang.org/protobuf/proto"
)

// ConfigFormat is a configurable format of Xray config file.
type ConfigFormat struct {
	Name      string
	Extension []string
	Loader    ConfigLoader
}

type ConfigSource struct {
	Name   string
	Format string
}

// ConfigLoader is a utility to load Xray config from external source.
type ConfigLoader func(input interface{}) (*Config, error)

// ConfigBuilder is a builder to build core.Config from filenames and formats
type ConfigBuilder func(files []*ConfigSource) (*Config, error)

// ConfigsMerger merges multiple json configs into a single one
type ConfigsMerger func(files []*ConfigSource) (string, error)

var (
	configLoaderByName    = make(map[string]*ConfigFormat)
	configLoaderByExt     = make(map[string]*ConfigFormat)
	ConfigBuilderForFiles ConfigBuilder
	ConfigMergedFormFiles ConfigsMerger
)

// RegisterConfigLoader add a new ConfigLoader.
func RegisterConfigLoader(format *ConfigFormat) error {
	name := strings.ToLower(format.Name)
	if _, found := configLoaderByName[name]; found {
		return errors.New(format.Name, " already registered.")
	}
	configLoaderByName[name] = format

	for _, ext := range format.Extension {
		lext := strings.ToLower(ext)
		if f, found := configLoaderByExt[lext]; found {
			return errors.New(ext, " already registered to ", f.Name)
		}
		configLoaderByExt[lext] = format
	}

	return nil
}

func GetMergedConfig(args cmdarg.Arg) (string, error) {
	var files []*ConfigSource
	supported := []string{"json", "yaml", "toml"}
	for _, file := range args {
		format := "json"
		if file != "stdin:" {
			format = GetFormat(file)
		}

		if slices.Contains(supported, format) {
			files = append(files, &ConfigSource{
				Name:   file,
				Format: format,
			})
		}
	}
	return ConfigMergedFormFiles(files)
}

func GetFormatByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case "pb", "protobuf":
		return "protobuf"
	case "yaml", "yml":
		return "yaml"
	case "toml":
		return "toml"
	case "json", "jsonc":
		return "json"
	default:
		return ""
	}
}

func getExtension(filename string) string {
	idx := strings.LastIndexByte(filename, '.')
	if idx == -1 {
		return ""
	}
	return filename[idx+1:]
}

func GetFormat(filename string) string {
	return GetFormatByExtension(getExtension(filename))
}

func LoadConfig(formatName string, input interface{}) (*Config, error) {
	switch v := input.(type) {
	case cmdarg.Arg:
		files := make([]*ConfigSource, len(v))
		hasProtobuf := false
		for i, file := range v {
			var f string

			if formatName == "auto" {
				if file != "stdin:" {
					f = GetFormat(file)
				} else {
					f = "json"
				}
			} else {
				f = formatName
			}

			if f == "" {
				return nil, errors.New("Failed to get format of ", file).AtWarning()
			}

			if f == "protobuf" {
				hasProtobuf = true
			}
			files[i] = &ConfigSource{
				Name:   file,
				Format: f,
			}
		}

		// only one protobuf config file is allowed
		if hasProtobuf {
			if len(v) == 1 {
				return configLoaderByName["protobuf"].Loader(v)
			} else {
				return nil, errors.New("Only one protobuf config file is allowed").AtWarning()
			}
		}

		// to avoid import cycle
		return ConfigBuilderForFiles(files)
	case io.Reader:
		if f, found := configLoaderByName[formatName]; found {
			return f.Loader(v)
		} else {
			return nil, errors.New("Unable to load config in", formatName).AtWarning()
		}
	}

	return nil, errors.New("Unable to load config").AtWarning()
}

func loadProtobufConfig(data []byte) (*Config, error) {
	config := new(Config)
	if err := proto.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func init() {
	common.Must(RegisterConfigLoader(&ConfigFormat{
		Name:      "Protobuf",
		Extension: []string{"pb"},
		Loader: func(input interface{}) (*Config, error) {
			switch v := input.(type) {
			case cmdarg.Arg:
				r, err := confloader.LoadConfig(v[0])
				common.Must(err)
				data, err := buf.ReadAllToBytes(r)
				common.Must(err)
				return loadProtobufConfig(data)
			case io.Reader:
				data, err := buf.ReadAllToBytes(v)
				common.Must(err)
				return loadProtobufConfig(data)
			default:
				return nil, errors.New("unknown type")
			}
		},
	}))
}
