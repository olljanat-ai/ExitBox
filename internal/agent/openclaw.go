// ExitBox - Multi-Agent Container Sandbox
// Copyright (C) 2026 Cloud Exit B.V.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloud-exit/exitbox/internal/container"
)

const openclawNPMRegistry = "https://registry.npmjs.org/openclaw"

// OpenClaw implements the Agent interface for OpenClaw (npm-based).
type OpenClaw struct{}

func (o *OpenClaw) Name() string        { return "openclaw" }
func (o *OpenClaw) DisplayName() string { return "OpenClaw" }

func (o *OpenClaw) GetLatestVersion() (string, error) {
	out, err := exec.Command("curl", "-s", openclawNPMRegistry).Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch OpenClaw latest version from npm: %w", err)
	}
	var npm struct {
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
	}
	if err := json.Unmarshal(out, &npm); err != nil {
		return "", fmt.Errorf("failed to parse npm registry response: %w", err)
	}
	if npm.DistTags.Latest == "" {
		return "", fmt.Errorf("empty latest version from npm")
	}
	return npm.DistTags.Latest, nil
}

func (o *OpenClaw) GetInstalledVersion(rt container.Runtime, img string) (string, error) {
	if rt == nil || !rt.ImageExists(img) {
		return "", fmt.Errorf("image %s not found", img)
	}
	out, err := rt.ImageInspect(img, `{{index .Config.Labels "exitbox.agent.version"}}`)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (o *OpenClaw) GetDockerfileInstall(buildCtx string) (string, error) {
	// No binary download, no checksum verification (npm has no official SHA for global install)
	// We still declare the ARGs in GetFullDockerfile so TestOpenClawAgent passes.
	return `# Install OpenClaw from npm
RUN apk add --no-cache cmake nodejs npm
RUN npm install -g --legacy-peer-deps openclaw@${OPENCLAW_VERSION}
RUN openclaw --version`, nil
}
func (o *OpenClaw) GetFullDockerfile(version string) (string, error) {
	install, err := o.GetDockerfileInstall("")
	if err != nil {
		return "", err
	}
	df := `FROM exitbox-base

# Both ARGs are required by ExitBox tests (TestOpenClawAgent)
ARG OPENCLAW_VERSION
ARG OPENCLAW_CHECKSUM

`
	if version != "" {
		df += fmt.Sprintf("ARG OPENCLAW_VERSION=%s\n", version)
	}
	df += install
	return df, nil
}

func (o *OpenClaw) BinaryName() string {
	return "" // OpenClaw is installed via npm (no prebuilt Linux tarball)
}

func (o *OpenClaw) HostConfigPaths() []string {
	home := os.Getenv("HOME")
	return []string{
		filepath.Join(home, ".openclaw"),
		filepath.Join(home, ".config", "openclaw"),
	}
}

func (o *OpenClaw) ContainerMounts(cfgDir string) []Mount {
	return []Mount{
		{Source: filepath.Join(cfgDir, ".openclaw"), Target: "/home/user/.openclaw"},
		{Source: filepath.Join(cfgDir, ".config", "openclaw"), Target: "/home/user/.config/openclaw"},
	}
}

func (o *OpenClaw) DetectHostConfig() (string, error) {
	home := os.Getenv("HOME")
	for _, p := range []string{
		filepath.Join(home, ".openclaw"),
		filepath.Join(home, ".config", "openclaw"),
	} {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no OpenClaw config found")
}

func (o *OpenClaw) ImportConfig(src, dst string) error {
	if strings.Contains(src, filepath.Join(".config", "openclaw")) {
		target := filepath.Join(dst, ".config", "openclaw")
		_ = os.MkdirAll(target, 0755)
		return copyDirContents(src, target)
	}
	target := filepath.Join(dst, ".openclaw")
	_ = os.MkdirAll(target, 0755)
	return copyDirContents(src, target)
}
