// Copyright 2017 The Prometheus Authors
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

package reload

import (
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/prometheus/config"
)

// Reload reload config
type Reload struct {
	logger     log.Logger
	mtx        sync.Mutex
	reloadConf *config.ReloadConfig
	goRun      bool          // 更新协程运行状态
	quitCh     chan struct{} // 退出程序
	updateCh   chan struct{} // 更新配置
	restartCh  chan struct{} // 重启协程
	reloadCh   chan struct{} // 加载配置
}

// NewReload 实例化Reload
func NewReload(logger log.Logger) *Reload {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	m := Reload{
		logger:    logger,
		quitCh:    make(chan struct{}),
		updateCh:  make(chan struct{}),
		restartCh: make(chan struct{}),
		reloadCh:  make(chan struct{}),
	}

	return &m
}

// ApplyConfig updates the config field of the Reload struct.
func (m *Reload) ApplyConfig(cfg *config.Config) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.reloadConf == nil {
		m.reloadConf = &cfg.ReloadConfig
	} else if cfg.ReloadConfig.Ticker != m.reloadConf.Ticker {
		m.reloadConf = &cfg.ReloadConfig
		m.updateCh <- struct{}{}
	}

	return nil
}

func (m *Reload) Reload() <-chan struct{} {
	return m.reloadCh
}

// reloader 更新配置
func (m *Reload) reloader() {
	ticker := time.NewTicker(time.Duration(m.reloadConf.Ticker))
	defer ticker.Stop()
	m.goRun = true
	defer func() { m.goRun = false }()

	for {
		select {
		case <-ticker.C:
			m.reloadCh <- struct{}{}
			level.Debug(m.logger).Log("msg", "auto reload once...")
		case <-m.restartCh:
			level.Debug(m.logger).Log("msg", "auto reload restart...")
			return
		case <-m.quitCh:
			level.Debug(m.logger).Log("msg", "auto reload stop...")
			return
		}
	}
}

// Run 开始运行reload
func (m *Reload) Run() error {
	level.Debug(m.logger).Log("msg", "reload run start...")
	reloader := func() {
		m.mtx.Lock()
		if m.reloadConf == nil {
			m.mtx.Unlock()
			return
		}
		tk := m.reloadConf.Ticker
		m.mtx.Unlock()
		if tk > 0 {
			m.reloader()
		}

	}
	go reloader()

LOOP:
	for {
		select {
		case <-m.updateCh:
			if m.goRun {
				m.restartCh <- struct{}{}
			}
			go reloader()

		case <-m.quitCh:
			break LOOP
		}
	}

	return nil
}

// Stop 退出SenderManager
func (m *Reload) Stop() {
	level.Debug(m.logger).Log("msg", "reload stop...")
	close(m.quitCh)
}
