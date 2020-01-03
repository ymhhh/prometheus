// Copyright 2020 The BDP
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
//         "CONTACT": ["huanghonghu"]
//         "USE_STATUS": ["上线"]

package mysql

import (
	"context"
	"fmt"
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
)

var (
	// DefaultSDConfig is the default file SD configuration.
	DefaultSDConfig = SDConfig{
		RefreshInterval: model.Duration(5 * time.Minute),
	}
)

// SDConfig is the configuration for file based discovery.
type SDConfig struct {
	DBConfig         map[string]interface{} `yaml:"database,omitempty"`
	RefreshInterval  model.Duration         `yaml:"refresh_interval,omitempty"`
	FilterConditions map[string][]string    `yaml:"filter_conditions,omitempty"`
}

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

	engines, err := txorm.NewXormEnginesFromConfig(tCfg, "mysql")
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
	for k, v := range p.cfg.FilterConditions {
		session = session.In(k, v)
	}

	date := time.Now().Format("2006-01-02")
	session = session.Where("`CREATE_DATE` = ?", date)

	var data []TKmServers
	if err := session.Find(&data); err != nil {
		return nil, err
	}

	serverExists := map[string]*targetgroup.Group{}

	for _, server := range data {
		server.parseTag()

		key := fmt.Sprintf("%s:%s:%s:%s",
			server.MachineRoom, server.Attribution, server.ClusterName, server.DeploymentServices)
		group := serverExists[key]
		if group == nil {
			group = &targetgroup.Group{
				Source: key,
			}
		}

		group.Targets = append(group.Targets,
			model.LabelSet{
				model.AddressLabel: model.LabelValue(fmt.Sprintf("%s:9100", server.IP)),
			})

		group.Labels = model.LabelSet{}

		if len(server.Attribution) > 0 {
			group.Labels["Attribution"] = server.Attribution
		}
		if len(server.ClusterName) > 0 {
			group.Labels["cluster"] = server.ClusterName
		}
		if len(server.UseStatus) > 0 {
			group.Labels["在线状态"] = server.UseStatus
		}
		if len(server.DeploymentServices) > 0 {
			group.Labels["服务"] = server.DeploymentServices
		}
		if len(server.MachineRoom) > 0 {
			group.Labels["机房"] = server.MachineRoom
		}

		if rsGroup, ok := server.tagMaps["RSGroup"]; ok {
			group.Labels["RSGroup"] = rsGroup
		}
		if product, ok := server.tagMaps["产品线"]; ok {
			group.Labels["产品线"] = product
		}
		if ascription, ok := server.tagMaps["归属"]; ok {
			group.Labels["归属"] = ascription
		}
		if clasterName, ok := server.tagMaps["集群名称"]; ok {
			group.Labels["集群名称"] = clasterName
		}

		serverExists[key] = group
	}

	var result []*targetgroup.Group
	for i := range serverExists {
		result = append(result, serverExists[i])
	}
	// level.Debug(p.logger).Log("msg", "Mysql discovery", "result", result)

	return result, nil
}
