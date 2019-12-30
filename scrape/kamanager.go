// Copyright 2015 The Prometheus Authors
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

package scrape

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"time"
)

type KaManager struct {
	logger    log.Logger
	append    Appendable
	graceShut chan struct{}

	jitterSeed    uint64     // Global jitterSeed seed is used to spread scrape workload across HA setup.
	mtxScrape     sync.Mutex // Guards the fields below.
	scrapeConfigs map[string]*config.ScrapeConfig
	scrapePools   map[string]*kaScrapePool
	targetSets    map[string][]*targetgroup.Group

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewKaManager(logger log.Logger, app Appendable) *KaManager {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &KaManager{
		append:        app,
		logger:        logger,
		scrapeConfigs: make(map[string]*config.ScrapeConfig),
		scrapePools:   make(map[string]*kaScrapePool),
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
}

// ApplyConfig resets the manager's target providers and job configurations as defined by the new cfg.
func (m *KaManager) ApplyConfig(cfg *config.Config) error {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	c := make(map[string]*config.ScrapeConfig)
	for _, scfg := range cfg.KaScrapeConfigs {
		c[scfg.JobName] = scfg
	}
	m.scrapeConfigs = c

	//if err := m.setJitterSeed(cfg.GlobalConfig.ExternalLabels); err != nil {
	//	return err
	//}

	// Cleanup and reload pool if the configuration has changed.
	var failed bool
	for name, sp := range m.scrapePools {
		if cfg, ok := m.scrapeConfigs[name]; !ok {
			sp.stop()
			delete(m.scrapePools, name)
		} else if !reflect.DeepEqual(sp.config, cfg) {
			//err := sp.reload(cfg)
			//if err != nil {
			//	level.Error(m.logger).Log("msg", "error reloading scrape pool", "err", err, "scrape_pool", name)
			//	failed = true
			//}
			sp.stop()
			delete(m.scrapePools, name)
			kaScrapeConfig, ok := m.scrapeConfigs[name]
			if !ok {
				level.Error(m.logger).Log("msg", "error reloading ka target set", "err", "invalid config id:"+name)
				failed = true
				continue
			}
			sp, err := newKaScrapePool(kaScrapeConfig, m.append, m.jitterSeed, log.With(m.logger, "kascrape_pool", name))
			if err != nil {
				level.Error(m.logger).Log("msg", "error creating new ka scrape pool", "err", err, "kascrape_pool", name)
				failed = true
				continue
			}
			m.scrapePools[name] = sp
			sp.Sync(m.targetSets[name])
		}
	}

	if failed {
		return errors.New("failed to apply the new ka configuration")
	}

	return nil
}

// Run receives and saves target set updates and triggers the scraping loops reloading.
// Reloading happens in the background so that it doesn't block receiving targets updates.
//func (m *KaManager) Run(tsets <-chan map[string][]*targetgroup.Group) error {
func (m *KaManager) Run(tsets <-chan map[string][]*targetgroup.Group) error {
	go m.reloader()

	for {
		select {
		case ts := <-tsets:
			m.updateTsets(ts)

			select {
			case m.triggerReload <- struct{}{}:
			default:
			}

		case <-m.graceShut:
			return nil
		}
	}
}

// Stop cancels all running scrape pools and blocks until all have exited.
func (m *KaManager) Stop() {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	for _, sp := range m.scrapePools {
		sp.stop()
	}

	close(m.graceShut)
}

func (m *KaManager) reloader() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.graceShut:
			return
		case <-ticker.C:
			select {
			case <-m.triggerReload:
				m.reload()
			case <-m.graceShut:
				return
			}
		}
	}
}

func (m *KaManager) reload() {
	m.mtxScrape.Lock()
	var wg sync.WaitGroup
	for setName, groups := range m.targetSets {
		if _, ok := m.scrapePools[setName]; !ok {
			kaScrapeConfig, ok := m.scrapeConfigs[setName]
			if !ok {
				level.Error(m.logger).Log("msg", "error reloading ka target set", "err", "invalid config id:"+setName)
				continue
			}
			sp, err := newKaScrapePool(kaScrapeConfig, m.append, m.jitterSeed, log.With(m.logger, "kascrape_pool", setName))
			if err != nil {
				level.Error(m.logger).Log("msg", "error creating new ka scrape pool", "err", err, "kascrape_pool", setName)
				continue
			}
			m.scrapePools[setName] = sp
		}

		wg.Add(1)
		// Run the sync in parallel as these take a while and at high load can't catch up.
		go func(sp *kaScrapePool, groups []*targetgroup.Group) {
			sp.Sync(groups)
			wg.Done()
		}(m.scrapePools[setName], groups)
	}
	m.mtxScrape.Unlock()
	wg.Wait()
}

func (m *KaManager) updateTsets(tsets map[string][]*targetgroup.Group) {
	m.mtxScrape.Lock()
	m.targetSets = tsets
	m.mtxScrape.Unlock()
}
