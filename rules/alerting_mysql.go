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

package rules

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-trellis/common/formats"
	"github.com/go-trellis/config"
	"github.com/go-trellis/txorm"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/models"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
)

func (m *Manager) initMysqlEngine(dbMap map[string]interface{}) error {
	if m.engine != nil {
		return nil
	}
	tCfg := config.MapGetter().GenMapConfig(config.ReaderTypeYAML, dbMap)

	engines, err := txorm.NewEnginesFromConfig(tCfg, "mysql")
	if err != nil {
		return err
	}

	m.engine = engines[txorm.DefaultDatabase]
	return nil
}

// UpdateMysql the rule manager's state as the config requires. If
// loading the new rules failed the old rule set is restored.
func (m *Manager) UpdateMysql(
	interval time.Duration, dbMap map[string]interface{}, externalLabels labels.Labels,
) error {

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if len(dbMap) == 0 {
		return nil
	}

	if err := m.initMysqlEngine(dbMap); err != nil {
		return err
	}

	groups, errs := m.LoadMysqlGroups(interval, externalLabels)
	if errs != nil {
		for _, e := range errs {
			level.Error(m.logger).Log("msg", "loading groups failed", "err", e)
		}
		return errors.New("error loading rules, previous rule set restored")
	}
	m.restored = true

	var wg sync.WaitGroup

	for _, newg := range groups {
		wg.Add(1)

		// If there is an old group with the same identifier, stop it and wait for
		// it to finish the current iteration. Then copy it into the new group.
		gn := groupKey(newg.name, newg.file)
		oldg, ok := m.groups[gn]
		delete(m.groups, gn)

		go func(newg *Group) {
			if ok {
				oldg.stop()
				newg.CopyState(oldg)
			}
			go func() {
				// Wait with starting evaluation until the rule manager
				// is told to run. This is necessary to avoid running
				// queries against a bootstrapping storage.
				<-m.block
				newg.run(m.opts.Context)
			}()
			wg.Done()
		}(newg)
	}

	// Stop remaining old groups.
	for _, oldg := range m.groups {
		oldg.stop()
	}

	wg.Wait()
	m.groups = groups

	return nil
}

// LoadMysqlGroups 加载Mysql的报警组
func (m *Manager) LoadMysqlGroups(
	interval time.Duration, externalLabels labels.Labels,
) (map[string]*Group, []error) {

	groups := make(map[string]*Group)
	shouldRestore := !m.restored

	var alerts []models.BzAlert

	if err := m.engine.Where("`is_using` = '1'").Find(&alerts); err != nil {
		return nil, []error{err}
	}

	for _, v := range alerts {
		itv := interval
		gkey := fmt.Sprintf("%s-%s", v.Id, v.Name)

		expr, err := promql.ParseExpr(v.Expression)
		if err != nil {
			return nil, []error{errors.Wrap(err, gkey)}
		}

		var alertThresholds []models.BzAlertThreshold

		if err := m.engine.Where("`alert_id` = ?", v.Id).Find(&alertThresholds); err != nil {
			return nil, []error{errors.Wrap(err, gkey)}
		}

		for _, thrd := range alertThresholds {
			rLabels := map[string]string{"monitor_id": v.Id, "severity": thrd.Severity}
			rule := NewAlertingRule(
				gkey,
				expr,
				formats.ParseStringTime(thrd.For),
				labels.FromMap(rLabels),
				labels.FromMap(nil),
				externalLabels,
				m.restored,
				log.With(m.logger, "alert_mysql", gkey),
			)

			groups[gkey] = NewGroup(v.Name, v.Id, itv, []Rule{rule}, shouldRestore, m.opts)
		}
	}

	return groups, nil
}
