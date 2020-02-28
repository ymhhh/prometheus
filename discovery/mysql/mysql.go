// Copyright 2020 The JD BDP
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// added by huanghonghu

// config sample
// scrape_configs:
// - job_name: 'node_exporter_kp'
//   honor_labels: true
//   honor_timestamps: true
//   mysql_sd_configs:
//     node_exporter_port: 9101
//     - database:
//         mysql:
//           ## database name
//           metastore_mysql:
//             host:      localhost
//             port:      3306
//             user:      root
//             password:  ""
//             charset:   utf8
//             location:  "Asia/Shanghai"
//             parseTime: "True"
//             allow_native_passwords: "true"
//             max_idle_conns: 10
//             max_open_conns: 100
//             log_level: 1
//             show_sql: true
//             is_default: true
//             timeout: 5s
//       refresh_interval: 10s
//       filter_conditions:
//         - key: "CLUSTER_NAME"
// 	 	     values: ["%haha%", "jdw"]
// 	 	     operator: like # =, like, in
//         - key: "USE_STATUS"
// 	 	     values: ["过保", "上线"]
// 	 	     operator: =
//         - key: "CONTACT"
// 	 	     values: ["huanghonghu"]
// 	 	     operator: in
//       tag_alias:
//         "在线状态": "online_status"
//         "服务器类型": "sever_type"
//         "内存": "memory"
//         "机房": "machine_root"
//         "产品线": "cluster_type"
//         "集群名称": "cluster_name"
//         "服务器型号": "model_type"
//         "cpu块": "cpu"
//         "归属": "ascription"
//         "saltmaster信息": "saltmaster"
//         "服务": "services"
//         "硬盘": "disk"
//         "负责人": "charge"

package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-trellis/config"
	"github.com/go-trellis/txorm"
	"github.com/go-xorm/xorm"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/refresh"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/models"
)

var (
	// DefaultSDConfig is the default file SD configuration.
	DefaultSDConfig = SDConfig{
		RefreshInterval: model.Duration(5 * time.Minute),
	}
)

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

// SDConfig is the configuration for file based discovery.
type SDConfig struct {
	DBConfig         DBConfig          `yaml:"database,omitempty"`
	RefreshInterval  model.Duration    `yaml:"refresh_interval,omitempty"`
	FilterConditions []FilterCondition `yaml:"filter_conditions,omitempty"`
	ServerPort       int               `yaml:"server_port,omitempty"`
	TagAlias         map[string]string `yaml:"tag_alias,omitempty"`
}

// FilterCondition 过滤器
type FilterCondition struct {
	Key      string   `yaml:"key,omitempty"`
	Values   []string `yaml:"values,omitempty"`
	Operator string   `yaml:"operator,omitempty"`
}

const (
	// FilterConditionEqual 数据库等于操作
	FilterConditionEqual = "="
	// FilterConditionLike 数据库Like操作
	FilterConditionLike = "like"
	// FilterConditionIn 数据库in操作
	FilterConditionIn = "in"
)

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSDConfig
	type plain SDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}
	if len(c.DBConfig) == 0 {
		return errors.New("mysql discovery config must contain at least one path name")
	}

	for _, v := range c.FilterConditions {
		if len(v.Key) == 0 || len(v.Values) == 0 {
			return errors.New("mysql discovery config: filter condition's key or values should not be empty")
		}
		switch v.Operator {
		case FilterConditionEqual, FilterConditionLike, FilterConditionIn:
		default:
			return errors.New("mysql discovery config: filter condition operator must: 'eq' or 'like' or 'in'")
		}
	}

	if (c.ServerPort) == 0 {
		c.ServerPort = 9100
	}
	return nil
}

// Discovery implements the Discoverer interface for discovering
// targets from Zookeeper.
type Discovery struct {
	engine *xorm.Engine

	*refresh.Discovery

	trellisConfig config.Config
	cfg           *SDConfig
	logger        log.Logger
}

// NewDiscovery returns a Discoverer function that calls a refresh() function at every interval.
func NewDiscovery(cfg *SDConfig, l log.Logger) (*Discovery, error) {
	if l == nil {
		l = log.NewNopLogger()
	}

	tCfg := config.MapGetter().GenMapConfig(config.ReaderTypeYAML, cfg.DBConfig)

	engines, err := txorm.NewEnginesFromConfig(tCfg, "mysql")
	if err != nil {
		return nil, err
	}

	d := &Discovery{
		cfg:           cfg,
		logger:        l,
		trellisConfig: tCfg,
		engine:        engines[txorm.DefaultDatabase],
	}

	d.Discovery = refresh.NewDiscovery(
		l,
		"mysql",
		time.Duration(cfg.RefreshInterval),
		d.refresh,
	)

	return d, nil
}

func (p *Discovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	defer level.Debug(p.logger).Log("msg", "Mysql discovery completed", "context", ctx)

	session := p.engine.NewSession()
	defer session.Close()

	date := time.Now().Format("2006-01-02")
	session = session.Where("`CREATE_DATE` = ?", date)
	for _, v := range p.cfg.FilterConditions {
		switch v.Operator {
		case FilterConditionIn:
			session = session.In(v.Key, v.Values)
		case FilterConditionEqual, FilterConditionLike:
			var wheres []string
			for _, cv := range v.Values {
				wheres = append(wheres, fmt.Sprintf("`%s` %s '%s'", v.Key, v.Operator, cv))
			}

			session = session.And(strings.Join(wheres, " or "))
		}
	}

	var data []models.TKmServers
	if err := session.Find(&data); err != nil {
		return nil, err
	}

	serverExists := map[string]*targetgroup.Group{}

	for _, server := range data {
		server.ParseTag()

		key := fmt.Sprintf("%s:%s:%s:%s",
			server.MachineRoom, server.Attribution, server.ClusterName, server.DeploymentServices)
		group := serverExists[key]
		if group == nil {
			group = &targetgroup.Group{
				Source: key,
			}
		}

		group.Targets = append(group.Targets,
			model.LabelSet{model.AddressLabel: model.LabelValue(fmt.Sprintf("%s:%d", server.IP, p.cfg.ServerPort))})

		group.Labels = model.LabelSet{}

		if len(server.Model) > 0 {
			group.Labels["model"] = server.Model
		}
		if len(server.Vendor) > 0 {
			group.Labels["vendor"] = server.Vendor
		}
		if len(server.MachineRoom) > 0 {
			group.Labels["machine_room"] = server.MachineRoom
		}
		if len(server.Cabinet) > 0 {
			group.Labels["cabinet"] = server.Cabinet
		}

		for tag, value := range server.TagMaps {
			aliasTag := p.tagAlias(string(tag))
			level.Debug(p.logger).Log("msg", "tag_alias", "ip", server.IP, "tag", tag, "aliasTag", aliasTag)

			if len(aliasTag) != 0 {
				if !aliasTag.IsValid() {
					level.Warn(p.logger).Log(
						"msg", "tag_alias_is_valid", "ip", server.IP, "tag", tag, "aliasTag", aliasTag)
					continue
				}
				group.Labels[aliasTag] = value
				continue
			}

			if !tag.IsValid() {
				level.Warn(p.logger).Log("msg", "tag_is_valid", "ip", server.IP, "tag", tag)
				continue
			}
			group.Labels[tag] = value
		}

		serverExists[key] = group
	}

	var result []*targetgroup.Group
	for i := range serverExists {
		result = append(result, serverExists[i])
	}

	return result, nil
}

func (p *Discovery) tagAlias(key string) model.LabelName {
	if p.cfg.TagAlias == nil {
		return model.LabelName(key)
	}

	return model.LabelName(p.cfg.TagAlias[key])
}
