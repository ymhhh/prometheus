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

// config sample
// scrape_configs:
// - job_name: 'blackbox_tcp'
//   metrics_path: /probe
//   params:
//     module: [tcp_connect]
//   relabel_configs:
//     - source_labels: [__address__]
//       target_label: __param_target
//     - source_labels: [__param_target]
//       target_label: instance
//     - target_label: __address__
//       replacement: 127.0.0.1:8080
//   blackbox_sd_configs:
//     - refresh_interval: 10m
//       type: tcp
//       database:
//         mysql:
//           ## database name
//           baize:
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
// - job_name: 'blackbox_http_2xx'
//   metrics_path: /probe
//   params:
//     module: [http_2xx]
//   relabel_configs:
//     - source_labels: [__address__]
//       target_label: __param_target
//     - source_labels: [__param_target]
//       target_label: instance
//     - target_label: __address__
//       replacement: 127.0.0.1:8080
//   blackbox_sd_configs:
//     - refresh_interval: 10m
//       type: http
//       database:
//         mysql:
//           ## database name
//           baize:
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

package bdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/prometheus/discovery/refresh"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/models"

	"github.com/go-trellis/config"
	"github.com/go-trellis/txorm"
	"github.com/go-xorm/xorm"
	"github.com/prometheus/common/model"
)

// BlackboxSDConfig is the configuration for file based discovery.
type BlackboxSDConfig struct {
	Type            string         `yaml:"type" json:"type"`
	RefreshInterval model.Duration `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`

	DBConfig DBConfig `yaml:"database,omitempty" json:"database,omitempty"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *BlackboxSDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultBlackboxSDConfig
	type plain BlackboxSDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}
	if len(c.DBConfig) == 0 {
		return errors.New("mysql discovery config must contain at least one path name")
	}
	return nil
}

// BlackboxDiscovery implements the Discoverer interface for discovering
// targets from Mysql Blackbox.
type BlackboxDiscovery struct {
	engine *xorm.Engine

	*refresh.Discovery

	cfg    *BlackboxSDConfig
	logger log.Logger
}

// NewBlackboxDiscovery returns a BlackboxDiscovery function that calls a refresh() function at every interval.
func NewBlackboxDiscovery(cfg *BlackboxSDConfig, l log.Logger) (*BlackboxDiscovery, error) {
	if l == nil {
		l = log.NewNopLogger()
	}

	tCfg := config.MapGetter().GenMapConfig(config.ReaderTypeYAML, cfg.DBConfig)

	engines, err := txorm.NewEnginesFromConfig(tCfg, "mysql")
	if err != nil {
		return nil, err
	}

	d := &BlackboxDiscovery{
		cfg:    cfg,
		logger: l,
		engine: engines[txorm.DefaultDatabase],
	}

	d.Discovery = refresh.NewDiscovery(
		l,
		"mysql",
		time.Duration(cfg.RefreshInterval),
		d.refresh,
	)

	return d, nil
}

func (p *BlackboxDiscovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	defer level.Debug(p.logger).Log("msg", "bdp blackbox mysql discovery completed", "context", ctx)

	var data []*models.BzAlertProbe
	if err := p.engine.Where("`type` = ?", p.cfg.Type).Find(&data); err != nil {
		return nil, err
	}

	var result []*targetgroup.Group

	for _, probe := range data {
		g := &targetgroup.Group{
			Labels: model.LabelSet{},
			Source: probe.Id,
		}
		instances := strings.Split(probe.Instances, ",")
		g.Labels["serviceId"] = model.LabelValue(probe.ServiceId)

		if probe.Labels != "" {
			var ls []MysqlLabel
			err := json.Unmarshal([]byte(probe.Labels), &ls)
			if err != nil {
				continue
			}
			for _, label := range ls {
				g.Labels[label.Label] = label.Value
			}
		}
		switch probe.Type {
		case "tcp":
			for _, instance := range instances {
				g.Targets = append(g.Targets, model.LabelSet{
					model.AddressLabel: model.LabelValue(instance),
				})
			}
		case "http":
			for _, instance := range instances {
				g.Targets = append(g.Targets, model.LabelSet{
					model.AddressLabel: model.LabelValue(fmt.Sprintf("http://%s", instance)),
				})
			}
		default:
			continue
		}
		result = append(result, g)
	}

	return result, nil
}
