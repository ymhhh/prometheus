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
	influx "github.com/influxdata/influxdb/client/v2"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/influxdb"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/opentsdb"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/prometheus/common/model"
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
	otsdbCli  tsdbWriter
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

	m.otsdbConf = &cfg.OtsdbConfigs
	if m.otsdbConf.TsdbName == "influxdb" {
		// influxdb
		influxConf := influx.HTTPConfig{
			Addr:     m.otsdbConf.TsdbAddr,
			Username: m.otsdbConf.UserName,
			Password: m.otsdbConf.Password,
			Timeout:  time.Duration(m.otsdbConf.WriteTimeout),
		}
		m.otsdbCli = influxdb.NewClient(m.logger, influxConf, m.otsdbConf.Database, m.otsdbConf.Retention)
	} else {
		// 默认opentsdb
		m.otsdbCli = opentsdb.NewClient(m.logger, m.otsdbConf.TsdbAddr, "", time.Duration(m.otsdbConf.IdleTimeout),
			time.Duration(m.otsdbConf.ConnTimeout), time.Duration(m.otsdbConf.WriteTimeout),
			false, m.otsdbConf.IsTelnet, m.otsdbConf.IsWait, m.otsdbConf.MaxConns, m.otsdbConf.MaxIdle)
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
	level.Info(m.logger).Log("msg", "SenderManager run",
		"TsdbName", m.otsdbConf.TsdbName, "TsdbAddr", m.otsdbConf.TsdbAddr,
		"ConnTimeout", m.otsdbConf.ConnTimeout, "WriteTimeout", m.otsdbConf.WriteTimeout,
		"MaxConns", m.otsdbConf.MaxConns, "MaxIdle", m.otsdbConf.MaxIdle,
		"IdleTimeout", m.otsdbConf.IdleTimeout, "BatchNum", m.otsdbConf.BatchNum,
		"BatchTimeout", m.otsdbConf.BatchTimeout, "MaxGoNum", m.otsdbConf.MaxGoNum,
		"RecvGoNum", m.otsdbConf.BatchTimeout)

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
		m.otsdbCli.Destroy()
	}
	close(m.graceShut)
}

type SenderReport struct {
	logger    log.Logger
	otsdbConf *config.OtsdbConfig
	graceShut chan struct{}
	cntMtx    sync.Mutex
	totalCnt  uint64
	failedCnt uint64
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
		failedCnt: 0,
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
func (r *SenderReport) addSendCnt(totalCnt, failedCnt uint64) {
	r.cntMtx.Lock()
	defer r.cntMtx.Unlock()
	r.totalCnt += totalCnt
	r.failedCnt += failedCnt
}

// report 统计发送数量
func (r *SenderReport) report() {
	r.cntMtx.Lock()
	totalCnt := r.totalCnt
	failedCnt := r.failedCnt
	r.totalCnt = 0
	r.failedCnt = 0
	r.cntMtx.Unlock()
	level.Debug(r.logger).Log("msg", "send records", "dateCnt", time.Now().Format("200601021504"), "totalCnt", totalCnt, "succCnt", totalCnt-failedCnt, "failedCnt", failedCnt)
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
	meFailed := model.Metric{
		"__name__": model.LabelValue(storageSamplesFailed),
		"instance": model.LabelValue(r.localIp),
		"port":     model.LabelValue(r.port),
	}
	for k, v := range r.otsdbConf.ReportLabels {
		meFailed[k] = v
	}
	sFailed := model.Sample{
		Metric:    meFailed,
		Value:     model.SampleValue(failedCnt),
		Timestamp: model.Time(t),
	}
	gOtsdbBuff <- &sFailed
}
