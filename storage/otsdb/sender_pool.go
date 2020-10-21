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
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/prometheus/prometheus/config"
)

// SenderPool 发送池
type SenderPool struct {
	logger    log.Logger
	otsdbConf *config.OtsdbConfig
	otsdbCli  map[string]tsdbWriter
	sr        *SenderReport

	batchTimeout time.Duration
	curGoNum     int // 当前处理协程数
	stCount      util.Count
	wg           sync.WaitGroup
	graceShut    chan struct{}
	recvGoNum    int
	waitChan     chan struct{}
}

// newSenderPool 实例化SenderPool
func newSenderPool(logger log.Logger, otsdbConf *config.OtsdbConfig, sr *SenderReport, otsdbCli map[string]tsdbWriter) *SenderPool {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	p := SenderPool{
		logger:       logger,
		otsdbConf:    otsdbConf,
		otsdbCli:     otsdbCli,
		sr:           sr,
		batchTimeout: time.Duration(otsdbConf.BatchTimeout),
		curGoNum:     0,
		graceShut:    make(chan struct{}),
	}

	if p.batchTimeout < 10*time.Second {
		p.batchTimeout = 10 * time.Second
	}

	// 同时接收数据的协程数
	p.recvGoNum = p.otsdbConf.RecvGoNum
	if p.recvGoNum == 0 || p.recvGoNum > p.otsdbConf.MaxGoNum {
		p.recvGoNum = p.otsdbConf.MaxGoNum
	}
	p.waitChan = make(chan struct{}, p.recvGoNum)

	return &p
}

// loop 循环启动run
func (p *SenderPool) loop() {
	level.Info(p.logger).Log("msg", "SenderPool loop start...")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 启动发送协程
	start := func() {
		for {
			curNum := p.stCount.Num()
			level.Info(p.logger).Log("curGoNum", curNum, "maxGoNum", p.otsdbConf.MaxGoNum)
			if curNum >= p.otsdbConf.MaxGoNum {
				break
			}

			p.stCount.Inc()
			p.wg.Add(1)
			go func() {
				defer p.stCount.Dec()
				defer p.wg.Done()
				p.run()
			}()
		}
	}
	start()
	<-time.After(3 * time.Second)
	for i := 0; i < p.recvGoNum; i++ {
		<-time.After(100 * time.Millisecond)
		p.waitChan <- struct{}{}
	}

LOOP:
	for {
		select {
		case <-ticker.C:
			start()
		case <-p.graceShut:
			break LOOP
		}
	}
	level.Info(p.logger).Log("msg", "SenderPool loop end...")
}

// run 开始运行SenderPool
func (p *SenderPool) run() {
	level.Info(p.logger).Log("msg", "SenderPool run start...")
	if gOtsdbBuff == nil {
		level.Error(p.logger).Log("msg", "gOtsdbBuff is nil")
		return
	}

	ticker := time.NewTicker(p.batchTimeout)
	defer ticker.Stop()

	samples := make(model.Samples, p.otsdbConf.BatchNum)
	pos := 0
	isWait := true
	lastTs := time.Now()

LOOP:
	for {
		if isWait {
			_, ok := <-p.waitChan
			if !ok {
				break LOOP
			}
			isWait = false
		}

		select {
		case s := <-gOtsdbBuff:
			samples[pos] = s
			pos++
			if pos == p.otsdbConf.BatchNum {
				p.waitChan <- struct{}{}
				isWait = true
				p.send(samples, pos)
				pos = 0
				samples = make(model.Samples, p.otsdbConf.BatchNum)
				lastTs = time.Now()
			}
		case <-ticker.C:
			p.waitChan <- struct{}{}
			isWait = true
			if pos > 0 && time.Since(lastTs) > p.batchTimeout {
				p.send(samples, pos)
				pos = 0
				samples = make(model.Samples, p.otsdbConf.BatchNum)
			}
		case <-p.graceShut:
			if pos > 0 {
				p.send(samples, pos)
			}
			break LOOP
		}
	}

	level.Info(p.logger).Log("msg", "SenderPool run end...")
}

// stop 退出SenderPool
func (p *SenderPool) stop() {
	level.Info(p.logger).Log("msg", "SenderPool stop...")
	close(p.waitChan)
	close(p.graceShut)
	p.wg.Wait()
}

// send 发送数据到tsdb
func (p *SenderPool) send(samples model.Samples, pos int) {
	if pos == 0 {
		return
	}

	cliLen := len(p.otsdbCli)
	if cliLen == 0 {
		level.Error(p.logger).Log("msg", "otsdbCli is empty")
		return
	}

	sendFunc := func(name string, cli tsdbWriter) {
		failedCnt, err := cli.Write(samples[:pos])
		if err != nil {
			level.Warn(p.logger).Log("msg", "Error sending samples to remote storage", "err", err, "num_samples", pos)
		}
		p.sr.addSendCnt(name, uint64(pos), uint64(failedCnt))
	}

	// send tsdb
	wg := sync.WaitGroup{}
	for otsdbName, otsdbCli := range p.otsdbCli {
		if cliLen == 1 {
			sendFunc(otsdbName, otsdbCli)
			return
		}

		wg.Add(1)
		go func(name string, cli tsdbWriter) {
			defer wg.Done()
			sendFunc(name, cli)
		}(otsdbName, otsdbCli)
	}

	if cliLen > 1 {
		wg.Wait()
	}
}
