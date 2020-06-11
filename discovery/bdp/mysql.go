/*
Copyright © 2020 BDP BAIZE <huanghonghu@jd.com>

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

package bdp

import (
	"encoding/json"
	"time"

	"github.com/prometheus/common/model"
)

var (
	// DefaultBlackboxSDConfig is the default blackbox SD configuration.
	DefaultBlackboxSDConfig = BlackboxSDConfig{
		RefreshInterval: model.Duration(30 * time.Minute),
	}
)

// MysqlLabel 标签
type MysqlLabel struct {
	Label    model.LabelName  `yaml:"label" json:"label"`
	Operater string           `yaml:"operator" json:"operator"`
	Value    model.LabelValue `yaml:"value" json:"value"`
}

// DBConfig 数据库配置对象
type DBConfig map[string]interface{}

const secretToken = "<secret>"

// MarshalYAML implements the yaml.Marshaler interface for Secret.
func (p DBConfig) MarshalYAML() (interface{}, error) {
	if p == nil || len(p) == 0 {
		return nil, nil
	}
	return map[string]interface{}{"database": secretToken}, nil
}

// MarshalJSON implements the json.Marshaler interface for Secret.
func (p DBConfig) MarshalJSON() ([]byte, error) {
	if p == nil || len(p) == 0 {
		return nil, nil
	}
	return json.Marshal(map[string]interface{}{"database": secretToken})
}
