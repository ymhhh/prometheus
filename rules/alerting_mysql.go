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
	"strings"
	"time"

	"github.com/prometheus/prometheus/models"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-trellis/common/formats"
	"github.com/go-trellis/config"
	"github.com/go-trellis/txorm"
	"github.com/pkg/errors"
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

	groups, err := m.LoadMysqlGroups(interval, externalLabels)
	if err != nil {
		level.Error(m.logger).Log("msg", "loading groups failed", "err", err)

		return errors.New("error loading rules, previous rule set restored")
	}

	m.runGroups(groups)

	return nil
}

// LoadMysqlGroups 加载Mysql的报警组
func (m *Manager) LoadMysqlGroups(
	interval time.Duration, externalLabels labels.Labels,
) (map[string]*Group, error) {

	shouldRestore := !m.restored

	var monitors []*models.BzMonitor
	if err := m.engine.Where("`is_using` = ?", 1).Find(&monitors); err != nil {
		return nil, err
	}

	groups := make(map[string]*Group)
	for _, monitor := range monitors {
		var monitorLabels []*models.BzMonitorLabels

		count, err := m.engine.Where("`monitor_id` = ?", monitor.Id).FindAndCount(&monitorLabels)
		if err != nil {
			return nil, err
		}

		if count == 0 {
			monitorRel := &models.BzMonitorRel{}

			mLabel := &models.BzMonitorLabels{
				Id: "default",
			}
			has, err := m.engine.Table("bz_monitor_rel").
				Join("INNER", "bz_monitor", "bz_monitor_rel.monitor_id = bz_monitor.id").
				Where("bz_monitor.id = ?", monitor.Id).
				Get(monitorRel)
			if err != nil {
				return nil, err
			} else if has {
				mLabel.LabelStr = fmt.Sprintf("serviceId=%q", monitorRel.RefId)
			}
			monitorLabels = append(monitorLabels, mLabel)
		}

		monitorGroups, err := m.loadMonitorGroups(monitor, monitorLabels, interval, externalLabels, shouldRestore)
		if err != nil {
			return nil, err
		}
		for key, value := range monitorGroups {
			groups[key] = value
		}
	}

	return groups, nil
}

func (m *Manager) loadMonitorGroups(
	monitor *models.BzMonitor, mLabels []*models.BzMonitorLabels,
	interval time.Duration, externalLabels labels.Labels, shouldRestore bool,
) (map[string]*Group, error) {

	groups := make(map[string]*Group)
	var alerts []*models.BzAlert
	if err := m.engine.Where("`is_using` = ?", 1).And("`monitor_id` = ?", monitor.Id).Find(&alerts); err != nil {
		return nil, err
	}

	for _, v := range alerts {
		itv := interval
		gkey := fmt.Sprintf("%s-%s", v.Name, v.Id)

		var thrds []*models.BzAlertThreshold
		if err := m.engine.Where("`alert_id` = ?", v.Id).Find(&thrds); err != nil {
			return nil, err
		}
		var alertRules []Rule

		for _, mLabel := range mLabels {
			rule, err := m.genAlertRules(v, thrds, mLabel, externalLabels)
			if err != nil {
				return nil, err
			}
			alertRules = append(alertRules, rule)
		}

		groups[gkey] = NewGroup(v.Name, v.Id, itv, alertRules, shouldRestore, m.opts)
	}

	return groups, nil
}

func (m *Manager) genAlertRules(
	alert *models.BzAlert,
	thrds []*models.BzAlertThreshold,
	mLabel *models.BzMonitorLabels,
	externalLabels labels.Labels,
) (Rule, error) {
	var alertLabels []models.BzAlertMetricLabel
	err := m.engine.Where("alert_id = ?", alert.Id).Find(&alertLabels)
	if err != nil {
		return nil, err
	}
	var labelStrs []string
	for _, l := range alertLabels {
		labelStrs = append(labelStrs, fmt.Sprintf(" %s%s%q ", l.Label, l.Operator, l.Value))
	}

	if mLabel.LabelStr != "" {
		labelStrs = append(labelStrs, mLabel.LabelStr)
	}

	labelStr := strings.Join(labelStrs, ",")

	var exprStrs []string
	for _, thrd := range thrds {

		exprStr := thrd.Metric
		if labelStr != "" {
			exprStr = fmt.Sprintf("%s{%s}", exprStr, labelStr)
		}

		switch thrd.Operator {
		case "between": // 在阈值范围内
			exprStr = fmt.Sprintf("%s >= %f and %s <= %f", exprStr, thrd.Threshold, exprStr, thrd.ThresholdMax)
		case "not_between": // 在阈值范围内
			exprStr = fmt.Sprintf("%s < %f or %s > %f", exprStr, thrd.Threshold, exprStr, thrd.ThresholdMax)
		default:
			exprStr = fmt.Sprintf("%s %s %f", exprStr, thrd.Operator, thrd.Threshold)
		}

		exprStrs = append(exprStrs, exprStr)
	}

	exprStr := strings.Join(exprStrs, " and ")

	expr, err := promql.ParseExpr(exprStr)
	if err != nil {
		level.Error(m.logger).Log("expression", exprStr, "err", err)
		return nil, err
	}
	rule := NewAlertingRule(
		alert.Id+":"+mLabel.Id,
		expr,
		formats.ParseStringTime(alert.For),
		labels.FromMap(map[string]string{
			"expression":       exprStr,
			"alert_id":         alert.Id,
			"monitor_label_id": mLabel.Id,
			"severity":         alert.Severity}),
		labels.FromMap(map[string]string{"summary": alert.Title, "description": alert.Content}),
		externalLabels,
		m.restored,
		log.With(m.logger, "alert_mysql", alert.Id, "expression", exprStr),
	)

	return rule, nil
}
