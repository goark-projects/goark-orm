package ormgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goark.dev/orm/internal/jsoncodec"
)

// GenerateConfig 描述可提交的 ORM 生成器配置文件。
type GenerateConfig struct {
	Dir          string                `json:"dir,omitempty"`
	PackageName  string                `json:"package,omitempty"`
	Output       string                `json:"output,omitempty"`
	DatabaseID   string                `json:"databaseId,omitempty"`
	TypeHandlers []string              `json:"typeHandlers,omitempty"`
	Packages     []GeneratePackageSpec `json:"packages,omitempty"`
}

// GeneratePackageSpec 描述配置文件中的单个 package 生成目标。
type GeneratePackageSpec struct {
	Dir          string   `json:"dir,omitempty"`
	PackageName  string   `json:"package,omitempty"`
	Output       string   `json:"output,omitempty"`
	DatabaseID   string   `json:"databaseId,omitempty"`
	TypeHandlers []string `json:"typeHandlers,omitempty"`
}

// ConfiguredGenerateSpec 表示已经完成默认值和路径解析的生成目标。
type ConfiguredGenerateSpec struct {
	Spec   GenerateSpec
	Output string
}

// LoadGenerateConfig 读取 JSON 生成器配置文件。
func LoadGenerateConfig(path string) (GenerateConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return GenerateConfig{}, fmt.Errorf("goark-orm: generate config path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return GenerateConfig{}, err
	}
	defer file.Close()
	var config GenerateConfig
	if err := jsoncodec.DecodeStrict(file, &config); err != nil {
		return GenerateConfig{}, fmt.Errorf("goark-orm: decode generate config %s failed: %w", path, err)
	}
	return config, nil
}

// Resolve 解析配置默认值和相对路径。
func (c GenerateConfig) Resolve(baseDir string) ([]ConfiguredGenerateSpec, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	baseDir = filepath.Clean(baseDir)
	packages := c.Packages
	if len(packages) == 0 {
		packages = []GeneratePackageSpec{{
			Dir:          c.Dir,
			PackageName:  c.PackageName,
			Output:       c.Output,
			DatabaseID:   c.DatabaseID,
			TypeHandlers: c.TypeHandlers,
		}}
	} else if len(packages) > 1 && strings.TrimSpace(c.Output) != "" {
		return nil, fmt.Errorf("goark-orm: top-level output cannot be used with multiple packages")
	}

	out := make([]ConfiguredGenerateSpec, 0, len(packages))
	for _, item := range packages {
		spec := GenerateSpec{
			Dir:          configFirstNonEmpty(item.Dir, c.Dir, "."),
			PackageName:  configFirstNonEmpty(item.PackageName, c.PackageName),
			DatabaseID:   configFirstNonEmpty(item.DatabaseID, c.DatabaseID),
			TypeHandlers: uniqueConfigStrings(append(append([]string(nil), c.TypeHandlers...), item.TypeHandlers...)),
		}
		output := strings.TrimSpace(item.Output)
		if output == "" && len(packages) == 1 {
			output = strings.TrimSpace(c.Output)
		}
		spec.Dir = resolveConfigPath(baseDir, spec.Dir)
		output = resolveConfigPath(baseDir, output)
		out = append(out, ConfiguredGenerateSpec{Spec: spec, Output: output})
	}
	return out, nil
}

func configFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func resolveConfigPath(baseDir string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func uniqueConfigStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
