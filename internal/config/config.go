/*
Copyright 2026.

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

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	envGitRepoURL           = "GIT_REPO_URL"
	envGitBranch            = "GIT_BRANCH"
	envBatchIntervalSeconds = "BATCH_INTERVAL_SECONDS"
	envExportPathTemplate   = "EXPORT_PATH_TEMPLATE"
	envGitUsername          = "GIT_USERNAME"
	envGitAccessToken       = "GIT_ACCESS_TOKEN"

	defaultGitBranch          = "main"
	defaultBatchIntervalSec   = 30
	defaultExportPathTemplate = "inventory/{{.PluralKind}}/{{.Namespace}}/{{.Name}}.yaml"
)

// InventoryControllerConfig holds non-flag settings for inventory-controller (12-factor env).
// Never log GitAccessToken or embed it in error strings returned to callers.
type InventoryControllerConfig struct {
	GitRepoURL         string
	GitBranch          string
	BatchInterval      time.Duration
	ExportPathTemplate string
	GitUsername        string
	GitAccessToken     string
}

// LoadInventoryControllerConfig reads environment variables with documented defaults.
func LoadInventoryControllerConfig() (InventoryControllerConfig, error) {
	var c InventoryControllerConfig
	c.GitRepoURL = os.Getenv(envGitRepoURL)
	c.GitBranch = os.Getenv(envGitBranch)
	if c.GitBranch == "" {
		c.GitBranch = defaultGitBranch
	}
	if v := os.Getenv(envBatchIntervalSeconds); v != "" {
		sec, err := strconv.Atoi(v)
		if err != nil {
			return InventoryControllerConfig{}, fmt.Errorf("parse %s: %w", envBatchIntervalSeconds, err)
		}
		if sec < 1 {
			return InventoryControllerConfig{}, fmt.Errorf("%s must be >= 1", envBatchIntervalSeconds)
		}
		c.BatchInterval = time.Duration(sec) * time.Second
	} else {
		c.BatchInterval = defaultBatchIntervalSec * time.Second
	}
	c.ExportPathTemplate = os.Getenv(envExportPathTemplate)
	if c.ExportPathTemplate == "" {
		c.ExportPathTemplate = defaultExportPathTemplate
	}
	c.GitUsername = os.Getenv(envGitUsername)
	c.GitAccessToken = os.Getenv(envGitAccessToken)
	return c, nil
}
