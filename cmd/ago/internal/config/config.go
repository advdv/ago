package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
)

const FileName = ".ago.yml"

type Config struct {
	Version string `yaml:"version" validate:"required,oneof=1"`
}

func Default() Config {
	return Config{
		Version: "1",
	}
}

type Loader interface {
	Load(path string) (Config, error)
}

type Writer interface {
	Write(w io.Writer, cfg Config) error
}

type Finder interface {
	Find(startDir string) (cfg Config, projectDir string, err error)
}

type yamlLoader struct {
	validate *validator.Validate
}

func NewLoader() Loader {
	return &yamlLoader{
		validate: validator.New(),
	}
}

func (l *yamlLoader) Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, errors.Wrap(err, "failed to read config file")
	}

	dec := yaml.NewDecoder(
		bytes.NewReader(data),
		yaml.Validator(l.validate),
		yaml.Strict(),
	)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrap(err, "failed to parse config file")
	}

	return cfg, nil
}

type yamlWriter struct{}

func NewWriter() Writer {
	return &yamlWriter{}
}

func (w *yamlWriter) Write(wr io.Writer, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}

	if _, err := wr.Write(data); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	return nil
}

type finder struct {
	loader Loader
}

func NewFinder(loader Loader) Finder {
	return &finder{loader: loader}
}

func (f *finder) Find(startDir string) (Config, string, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, FileName)
		if _, err := os.Stat(configPath); err == nil {
			cfg, err := f.loader.Load(configPath)
			if err != nil {
				return Config{}, "", err
			}
			return cfg, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, "", errors.Newf(
				"config file %s not found (searched from %s to root)",
				FileName, startDir,
			)
		}
		dir = parent
	}
}

func WriteToFile(dir string, cfg Config, w Writer) error {
	path := filepath.Join(dir, FileName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer f.Close()

	return w.Write(f, cfg)
}
