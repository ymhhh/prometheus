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
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/refresh"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/models"
	"github.com/prometheus/prometheus/util"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-trellis/config"
	"github.com/go-trellis/txorm"
	"github.com/go-xorm/xorm"
)

// BlackboxSDConfig is the configuration for file based discovery.
type BlackboxSDConfig struct {
	ProbeName       string         `yaml:"probe_name"`
	Type            string         `yaml:"type"`
	RefreshInterval model.Duration `yaml:"refresh_interval,omitempty"`

	DBConfig DBConfig `yaml:"database,omitempty"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *BlackboxSDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultBlackboxSDConfig
	type plain BlackboxSDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
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

	tagsCluster map[int64]string
	lastRefresh map[string]int
}

// NewBlackboxDiscovery returns a BlackboxDiscovery function that calls a refresh() function at every interval.
func NewBlackboxDiscovery(cfg *BlackboxSDConfig, dbConfig *DBConfig, l log.Logger) (*BlackboxDiscovery, error) {
	if l == nil {
		l = log.NewNopLogger()
	}

	if len(cfg.DBConfig) == 0 {
		cfg.DBConfig = *dbConfig
	}
	tCfg := config.MapGetter().GenMapConfig(config.ReaderTypeYAML, cfg.DBConfig)

	engines, err := txorm.NewEnginesFromConfig(tCfg, "mysql")
	if err != nil {
		return nil, err
	}

	d := &BlackboxDiscovery{
		cfg:         cfg,
		logger:      l,
		engine:      engines[txorm.DefaultDatabase],
		tagsCluster: make(map[int64]string),
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

	p.tagsCluster = make(map[int64]string)
	var data []*models.BzAlertProbe
	if err := p.engine.Where("`type` = ?", p.cfg.Type).Find(&data); err != nil {
		return nil, err
	}

	var result []*targetgroup.Group
	curRefresh := map[string]int{}

	for _, probe := range data {
		instances := strings.Split(probe.Instances, ",")
		gLabels := model.LabelSet{}

		// labels
		if probe.AddLabels && probe.Labels != "" {
			var ls model.LabelSet
			err := json.Unmarshal([]byte(probe.Labels), &ls)
			if err != nil {
				level.Debug(p.logger).Log("msg", "Unmarshal err", "err", err.Error(), "labels", probe.Labels)
				continue
			}
			for label, value := range ls {
				gLabels[label] = value
			}
		}

		// tags
		if probe.Tags != "" {
			var ts model.LabelSet
			err := json.Unmarshal([]byte(probe.Tags), &ts)
			if err != nil {
				level.Debug(p.logger).Log("msg", "Unmarshal err", "err", err.Error(), "tags", probe.Tags)
				continue
			}
			gLabels["product"] = ts["product"]
			gLabels["deploymentServices"] = ts["deploymentServices"]
		}

		mGroups := map[string]*targetgroup.Group{}
		for _, instance := range instances {
			if len(instance) == 0 {
				continue
			}
			tagKey := "_"
			cluster := ""
			tags := model.LabelSet{}
			if probe.Tags != "" {
				arr := strings.Split(instance, ":")
				sIp := arr[0]
				cluster = p.getClusterByIp(sIp)
				if cluster != "" {
					tagKey = cluster
					tags = model.LabelSet{"cluster": model.LabelValue(cluster)}
				}
			}
			if probe.Type == "http" {
				instance = fmt.Sprintf("http://%s", instance)
			}
			if _, ok := mGroups[tagKey]; ok {
				mGroups[tagKey].Targets = append(mGroups[tagKey].Targets, model.LabelSet{model.AddressLabel: model.LabelValue(instance)})
			} else {
				source := fmt.Sprintf("%s.%s", probe.Id, tagKey)
				curRefresh[source] = 1
				g := targetgroup.Group{
					Targets: []model.LabelSet{{model.AddressLabel: model.LabelValue(instance)}},
					Labels:  tags.Merge(gLabels),
					Source:  source,
				}
				mGroups[tagKey] = &g
			}
		}

		for _, g := range mGroups {
			result = append(result, g)
		}
	}

	// 清除老的target
	for k := range p.lastRefresh {
		if _, ok := curRefresh[k]; ok {
			continue
		}

		g := targetgroup.Group{
			Targets: []model.LabelSet{},
			Labels:  model.LabelSet{},
			Source:  k,
		}
		result = append(result, &g)
	}
	p.lastRefresh = curRefresh

	return result, nil
}

// getClusterByIp 根据IP获取cluster
func (p *BlackboxDiscovery) getClusterByIp(sIp string) string {
	ipNum := util.IpAtoi(sIp)
	if cluster, ok := p.tagsCluster[ipNum]; ok {
		return cluster
	}

	var data []*models.BzOpsIps
	if err := p.engine.Where("`ip_num` = ? and `tag_key` = 'cluster'", ipNum).Limit(1).Find(&data); err != nil {
		level.Error(p.logger).Log("msg", "get bz_ops_ips data err", "err", err.Error())
		return ""
	} else if len(data) == 0 {
		return ""
	}

	cluster := ""
	for _, v := range data {
		cluster = v.TagValue
		break
	}
	p.tagsCluster[ipNum] = cluster

	return cluster
}
