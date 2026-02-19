package workspace

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".peekgit.yaml"

type WorkspaceConfig struct {
	Repos []string `yaml:"repos"`
}

// LoadConfig 从 workspace 根目录读取 .peekgit.yaml。
// 文件不存在时返回空配置而非错误。
func LoadConfig(root string) (WorkspaceConfig, error) {
	cfgPath := filepath.Join(root, ConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceConfig{}, nil
		}
		return WorkspaceConfig{}, err
	}

	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return WorkspaceConfig{}, err
	}
	return cfg, nil
}
