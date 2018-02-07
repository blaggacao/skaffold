/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docker

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/moby/moby/pkg/homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	configFileDir = ".docker"
)

var (
	configDir = os.Getenv("DOCKER_CONFIG")
)

// DefaultAuthHelper is exposed so that other packages can override it for testing
var DefaultAuthHelper AuthConfigHelper

func init() {
	DefaultAuthHelper = credsHelper{}

	if configDir == "" {
		configDir = filepath.Join(homedir.Get(), configFileDir)
	}
}

// AuthConfigHelper exists for testing purposes ssince GetAuthConfig shells out
// to native store helpers.s
// Ideally this shouldn't be public, but the LocalBuilder needs to use it.
type AuthConfigHelper interface {
	GetAuthConfig(registry string) (types.AuthConfig, error)
	GetAllAuthConfigs() (map[string]types.AuthConfig, error)
}

type credsHelper struct {
	cf *configfile.ConfigFile
}

func (credsHelper) GetAuthConfig(registry string) (types.AuthConfig, error) {
	cf, err := load()
	if err != nil {
		return types.AuthConfig{}, errors.Wrap(err, "docker config")
	}
	return cf.GetAuthConfig(registry)
}

func (credsHelper) GetAllAuthConfigs() (map[string]types.AuthConfig, error) {
	cf, err := load()
	if err != nil {
		return nil, errors.Wrap(err, "docker config")
	}
	spew.Dump(cf)
	return cf.GetAllCredentials()
}

func encodedRegistryAuth(a AuthConfigHelper, image string) (string, error) {
	ref, err := reference.ParseNamed(image)
	if err == reference.ErrNameNotCanonical {
		logrus.Infof("Image %s not canonical, skipping registry auth helpers", image)
		return "", nil
	}
	if err != nil {
		return "", errors.Wrap(err, "parsing image name for registry")
	}
	registry := reference.Domain(ref)

	ac, err := a.GetAuthConfig(registry)
	if err != nil {
		return "", errors.Wrap(err, "getting auth config")
	}

	buf, err := json.Marshal(ac)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

func load() (*configfile.ConfigFile, error) {
	filename := filepath.Join(configDir, config.ConfigFileName)
	f, err := fs.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "opening docker config")
	}
	defer f.Close()
	cf := configfile.New("")
	if err := cf.LoadFromReader(f); err != nil {
		return nil, errors.Wrap(err, "loading docker config file")
	}
	return cf, nil
}
