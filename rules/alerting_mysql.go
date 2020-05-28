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

		monitorRel := &models.BzMonitorRel{}

		serviceID := ""
		has, err := m.engine.Table("bz_monitor_rel").
			Join("INNER", "bz_monitor", "bz_monitor_rel.monitor_id = bz_monitor.id").
			Where("bz_monitor.id = ?", monitor.Id).
			Get(monitorRel)
		if err != nil {
			return nil, err
		} else if has {
			serviceID = monitorRel.RefId
		}
		rules, err := m.loadMonitorAlertRules(serviceID, monitor, externalLabels)
		if err != nil {
			return nil, err
		}
		if len(rules) > 0 {
			groups[monitor.Id] = NewGroup(monitor.Id, serviceID, interval, rules, shouldRestore, m.opts)
		}
	}

	return groups, nil
}

func (m *Manager) loadMonitorAlertRules(
	serviceID string,
	monitor *models.BzMonitor,
	externalLabels labels.Labels,
) ([]Rule, error) {

	var alerts []*models.BzAlert
	if err := m.engine.Where("`is_using` = ? and `monitor_id` = ?", 1, monitor.Id).Find(&alerts); err != nil {
		return nil, err
	}

	var alertsRules []Rule
	for _, v := range alerts {
		rule, err := m.genAlertRules(serviceID, monitor, v, externalLabels)
		if err != nil {
			return nil, err
		}
		alertsRules = append(alertsRules, rule)
	}

	return alertsRules, nil
}

// 告警规则的阈值类型
const (
	// 白泽类型，拆分规则到细粒度的指标名
	ThresholdTypeBaize int32 = iota
	// 用户自定义类型，用户自己传表达式
	ThresholdTypeCustomer
)

func (m *Manager) genAlertRules(
	serviceID string,
	monitor *models.BzMonitor,
	alert *models.BzAlert,
	externalLabels labels.Labels,
) (Rule, error) {

	var thrds []*models.BzAlertThreshold
	if err := m.engine.Where("`alert_id` = ?", alert.Id).Find(&thrds); err != nil {
		return nil, err
	}

	var actions []*models.BzAlertAction
	if err := m.engine.Where("`alert_id` = ?", alert.Id).Find(&actions); err != nil {
		return nil, err
	}

	var exprStrs, thresholds []string
	for _, thrd := range thrds {
		exprStr := thrd.Metric
		switch alert.ThresholdType {
		case ThresholdTypeCustomer:
			// 什么都不做
		default:
			var alertLabels []*models.BzAlertMetricLabel
			err := m.engine.Where("`alert_id` = ? and `metric` = ?", alert.Id, thrd.Metric).Find(&alertLabels)
			if err != nil {
				return nil, err
			}

			var labelStrs []string
			if serviceID != "" {
				labelStrs = append(labelStrs, fmt.Sprintf("serviceId=%q", serviceID))
			}

			for _, l := range alertLabels {
				labelStrs = append(labelStrs, fmt.Sprintf("%s%s%q", l.Label, l.Operator, l.Value))
			}

			labelStr := strings.Join(labelStrs, ", ")
			if labelStr != "" {
				exprStr = fmt.Sprintf("%s{%s}", exprStr, labelStr)
			}

			if thrd.RangeVectors != "" {
				exprStr = fmt.Sprintf("%s[%s]", exprStr, thrd.RangeVectors)
			}

			if thrd.Offset != "" {
				exprStr = fmt.Sprintf("%s offset %s", exprStr, thrd.Offset)
			}

			if thrd.ComputeFunction != "" {
				// TODO
				// 比较Low，只支持上云的2级函数，以后再想解决方案
				num := strings.Count(thrd.ComputeFunction, "(")
				exprStr = fmt.Sprintf("%s(%s)%s", thrd.ComputeFunction, exprStr, strings.Repeat(")", num))
			}

			if thrd.ByLabels != "" {
				exprStr = fmt.Sprintf("%s by (%s)", exprStr, thrd.ByLabels)
			}

		}

		thresholdStr := ""
		switch thrd.Operator {
		case "between": // 在阈值范围内
			exprStr = fmt.Sprintf("%s >= %f and %s <= %f", exprStr, thrd.Threshold, exprStr, thrd.ThresholdMax)
			thresholdStr = fmt.Sprintf("%f,%f", thrd.Threshold, thrd.ThresholdMax)
		case "not_between": // 在阈值范围内
			exprStr = fmt.Sprintf("%s < %f or %s > %f", exprStr, thrd.Threshold, exprStr, thrd.ThresholdMax)
			thresholdStr = fmt.Sprintf("%f,%f", thrd.Threshold, thrd.ThresholdMax)
		default:
			exprStr = fmt.Sprintf("%s %s %f", exprStr, thrd.Operator, thrd.Threshold)
			thresholdStr = fmt.Sprintf("%f", thrd.Threshold)
		}
		exprStrs = append(exprStrs, exprStr)
		thresholds = append(thresholds, thresholdStr)
	}

	exprStr := strings.Join(exprStrs, " and ")

	expr, err := promql.ParseExpr(exprStr)
	if err != nil {
		level.Error(m.logger).Log("expression", exprStr, "err", err)
		return nil, err
	}

	labelsMap := map[string]string{
		"expression":   exprStr,
		"alert_id":     alert.Id,
		"alert_name":   alert.Name,
		"severity":     alert.Severity,
		"monitor_name": monitor.Name,
		"threshold":    strings.Join(thresholds, ";"),
	}

	annotations := map[string]string{
		"summary":     alert.Title,
		"description": alert.Content,
		"value":       "{{$value}}",
	}

	// 将action的内容做转换
	for _, action := range actions {
		annotations[strings.Replace(action.Id, "-", "_", -1)] = action.Content
	}

	rule := NewAlertingRule(
		alert.Id,
		expr,
		formats.ParseStringTime(alert.For),
		labels.FromMap(labelsMap),
		labels.FromMap(annotations),
		externalLabels,
		m.restored,
		log.With(m.logger, "alert_mysql", alert.Id, "expression", exprStr),
	)

	return rule, nil
}
