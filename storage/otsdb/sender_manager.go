// Copyright 2020 The JD Authors
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

package otsdb

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	influx "github.com/influxdata/influxdb/client/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/influxdb"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/opentsdb"

	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/util"
)

var (
	storageSamplesTotal  = "storage_samples_total"
	storageSamplesFailed = "storage_samples_failed"
)

type tsdbWriter interface {
	Write(samples model.Samples) (int, error)
	Name() string
	Destroy()
}

// SenderManager 发送管理端
type SenderManager struct {
	logger    log.Logger
	addr      string
	otsdbConf *config.OtsdbConfig
	otsdbCli  map[string]tsdbWriter
	mtxOtsdb  sync.Mutex
	sp        *SenderPool
	sr        *SenderReport
	graceShut chan struct{}
}

// NewSenderManager 实例化SenderManager
func NewSenderManager(logger log.Logger, addr string) *SenderManager {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	m := SenderManager{
		logger:    logger,
		addr:      addr,
		graceShut: make(chan struct{}),
	}

	return &m
}

// ApplyConfig updates the config field of the SenderManager struct.
func (m *SenderManager) ApplyConfig(cfg *config.Config) error {
	m.mtxOtsdb.Lock()
	defer m.mtxOtsdb.Unlock()

	// 暂时不支持reload
	if m.otsdbCli != nil {
		return nil
	}

	m.otsdbConf = &cfg.OtsdbConfigs
	m.otsdbCli = make(map[string]tsdbWriter)
	for _, storageCfg := range cfg.OtsdbConfigs.StorageConfigs {
		if storageCfg.TsdbType == "influxdb" {
			// influxdb
			influxConf := influx.HTTPConfig{
				Addr:     storageCfg.TsdbAddr,
				Username: storageCfg.UserName,
				Password: storageCfg.Password,
				Timeout:  time.Duration(storageCfg.WriteTimeout),
			}
			m.otsdbCli[storageCfg.TsdbName] = influxdb.NewClient(m.logger, influxConf, storageCfg.Database, storageCfg.Retention)
		} else if storageCfg.TsdbType == "remote" {
			// remote
			m.otsdbCli[storageCfg.TsdbName] = NewRemoteClient(m.logger, storageCfg.TsdbAddr, time.Duration(storageCfg.WriteTimeout), storageCfg.Headers)
		} else {
			// 默认opentsdb
			m.otsdbCli[storageCfg.TsdbName] = opentsdb.NewClient(m.logger, storageCfg.TsdbAddr, "", time.Duration(storageCfg.IdleTimeout),
				time.Duration(storageCfg.ConnTimeout), time.Duration(storageCfg.WriteTimeout),
				false, storageCfg.IsTelnet, storageCfg.IsWait, storageCfg.MaxConns, storageCfg.MaxIdle, 0)
		}
	}

	m.sr = newSenderReport(m.logger, m.otsdbConf, m.addr)
	m.sp = newSenderPool(m.logger, m.otsdbConf, m.sr, m.otsdbCli)

	return nil
}

// Run 开始运行SenderManager
func (m *SenderManager) Run() error {
	if gOtsdbBuff == nil {
		level.Error(m.logger).Log("msg", "gOtsdbBuff is nil")
		return errors.New("gOtsdbBuff is nil")
	}
	if m.otsdbConf == nil {
		level.Error(m.logger).Log("msg", "otsdbConf is nil")
		return errors.New("otsdbConf is nil")
	}
	for _, storageCfg := range m.otsdbConf.StorageConfigs {
		level.Info(m.logger).Log("msg", "SenderManager run",
			"TsdbName", storageCfg.TsdbName, "TsdbType", storageCfg.TsdbType, "TsdbAddr", storageCfg.TsdbAddr,
			"ConnTimeout", storageCfg.ConnTimeout, "WriteTimeout", storageCfg.WriteTimeout,
			"MaxConns", storageCfg.MaxConns, "MaxIdle", storageCfg.MaxIdle,
			"IdleTimeout", storageCfg.IdleTimeout)
	}
	level.Info(m.logger).Log("msg", "SenderManager run",
		"BatchNum", m.otsdbConf.BatchNum, "BatchTimeout", m.otsdbConf.BatchTimeout,
		"MaxGoNum", m.otsdbConf.MaxGoNum, "RecvGoNum", m.otsdbConf.RecvGoNum)

	go m.sr.run()
	go m.sp.loop()

	<-m.graceShut
	return nil
}

// Stop 退出SenderManager
func (m *SenderManager) Stop() {
	if m.sr != nil {
		m.sr.stop()
	}
	if m.sp != nil {
		m.sp.stop()
	}
	if m.otsdbCli != nil {
		for _, cli := range m.otsdbCli {
			cli.Destroy()
		}
	}
	close(m.graceShut)
}

type SenderReport struct {
	logger    log.Logger
	otsdbConf *config.OtsdbConfig
	graceShut chan struct{}
	cntMtx    sync.Mutex
	totalCnt  uint64
	failedCnt map[string]uint64
	localIp   string
	port      string
}

func newSenderReport(logger log.Logger, otsdbConf *config.OtsdbConfig, addr string) *SenderReport {
	localIp, _ := util.ExternalIP()
	if logger == nil {
		logger = log.NewNopLogger()
	}
	port := ""
	if addr != "" {
		arr := strings.Split(addr, ":")
		if len(arr) > 1 {
			port = arr[1]
		} else {
			port = arr[0]
		}
	}
	r := SenderReport{
		logger:    logger,
		otsdbConf: otsdbConf,
		graceShut: make(chan struct{}),
		totalCnt:  0,
		failedCnt: make(map[string]uint64),
		localIp:   localIp,
		port:      port,
	}

	return &r
}

// report 上报发送数据
func (r *SenderReport) run() {
	level.Info(r.logger).Log("msg", "SenderReport run start...")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

LOOP:
	for {
		select {
		case <-ticker.C:
			r.report()
		case <-r.graceShut:
			r.report()
			break LOOP
		}
	}

	level.Info(r.logger).Log("msg", "SenderReport run end...")
}

// stop 通过程序
func (r *SenderReport) stop() {
	level.Info(r.logger).Log("msg", "SenderReport stop...")
	close(r.graceShut)
}

// addTotalCnt 添加sample发送数量
func (r *SenderReport) addSendCnt(tsdbName string, totalCnt, failedCnt uint64) {
	r.cntMtx.Lock()
	defer r.cntMtx.Unlock()
	r.totalCnt += totalCnt
	if _, ok := r.failedCnt[tsdbName]; !ok {
		r.failedCnt[tsdbName] = failedCnt
	} else {
		r.failedCnt[tsdbName] += failedCnt
	}
}

// report 统计发送数量
func (r *SenderReport) report() {
	r.cntMtx.Lock()
	totalCnt := r.totalCnt
	failedCnt := r.failedCnt
	r.totalCnt = 0
	r.failedCnt = make(map[string]uint64)
	r.cntMtx.Unlock()
	t := time.Now().Unix() * 1000

	// sample总数
	meTotal := model.Metric{
		"__name__": model.LabelValue(storageSamplesTotal),
		"instance": model.LabelValue(r.localIp),
		"port":     model.LabelValue(r.port),
	}
	for k, v := range r.otsdbConf.ReportLabels {
		meTotal[k] = v
	}
	sTotal := model.Sample{
		Metric:    meTotal,
		Value:     model.SampleValue(totalCnt),
		Timestamp: model.Time(t),
	}
	gOtsdbBuff <- &sTotal

	// sample失败数
	for tsdbName, cnt := range failedCnt {
		level.Debug(r.logger).Log("msg", "send records", "dateCnt", time.Now().Format("200601021504"), "tsdbName", tsdbName, "totalCnt", totalCnt, "succCnt", totalCnt-cnt, "failedCnt", cnt)
		meFailed := model.Metric{
			"__name__":  model.LabelValue(storageSamplesFailed),
			"instance":  model.LabelValue(r.localIp),
			"port":      model.LabelValue(r.port),
			"tsdb_name": model.LabelValue(tsdbName),
		}
		for k, v := range r.otsdbConf.ReportLabels {
			meFailed[k] = v
		}
		sFailed := model.Sample{
			Metric:    meFailed,
			Value:     model.SampleValue(cnt),
			Timestamp: model.Time(t),
		}
		gOtsdbBuff <- &sFailed
	}
}
