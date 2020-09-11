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
	"context"
	"crypto/sha256"
	"hash"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util"
	"github.com/xdg/scram"
)

var (
	SHA256 scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
)

const (
	GZIP_V01 = "G01_"
)

type MetricData map[string]interface{}

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}

// kaScrapePool manages scrapes for sets of targets.
type kaScrapePool struct {
	appendable storage.Appendable
	logger     log.Logger
	config     *config.ScrapeConfig
	mtx        sync.RWMutex

	maxGoNums int        // max goroutine number
	curGoNums int        // current goroutine number
	lockNum   sync.Mutex // lock for goroutine
	wg        sync.WaitGroup
	isStop    chan bool
	isReload  chan bool
	isChange  bool // 配置是否修改

	kaCfg    *sarama.Config
	kaCtx    context.Context
	kaCancel context.CancelFunc
	message  chan string

	lockCnt   sync.Mutex
	dateCnt   string
	totalCnt  int64 // 目标暴露的样本数(scrape_samples_scraped)
	addedCnt  int64 // 应用度量标准重新标记后剩余的样本数(scrape_samples_post_metric_relabeling)
	seriesCnt int64 // 该刮擦中新系列的大致数量(scrape_series_added)
	ca        *scrapeCache
}

func newKaScrapePool(cfg *config.ScrapeConfig, app storage.Appendable, jitterSeed uint64, isTsdb bool, logger log.Logger) (*kaScrapePool, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	msgLen := cfg.MsgChanLen
	if msgLen <= 0 {
		msgLen = cfg.MaxGoNum
	}

	sp := kaScrapePool{
		config:     cfg,
		appendable: app,
		maxGoNums:  cfg.MaxGoNum,
		curGoNums:  0,
		isStop:     make(chan bool),
		isReload:   make(chan bool),
		isChange:   true,
		message:    make(chan string, msgLen),
		logger:     logger,
		dateCnt:    time.Now().Format("2006010215"),
		totalCnt:   0,
		addedCnt:   0,
		seriesCnt:  0,
		ca:         newScrapeCache(true, isTsdb),
	}

	// kafka config
	sp.kaCfg = sarama.NewConfig()
	sp.kaCfg.Consumer.Return.Errors = true
	//sp.kaCfg.Group.Return.Notifications = true
	sp.kaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	if sp.config.KaConfig.KaUser != "" {
		sp.kaCfg.Net.SASL.Enable = true
		sp.kaCfg.Net.SASL.User = sp.config.KaConfig.KaUser
		sp.kaCfg.Net.SASL.Password = sp.config.KaConfig.KaPwd
		sp.kaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		sp.kaCfg.Net.SASL.Mechanism = sarama.SASLMechanism(sarama.SASLTypeSCRAMSHA256)
	}

	if sp.config.KaConfig.KaVer != "" {
		version, err := sarama.ParseKafkaVersion(sp.config.KaConfig.KaVer)
		if err != nil {
			level.Error(sp.logger).Log("msg", "parsing Kafka version error", "version", sp.config.KaConfig.KaVer, "err", err.Error())
			panic("parsing Kafka version error")
		}
		sp.kaCfg.Version = version
	}
	if sp.config.KaConfig.KaClientId != "" {
		sp.kaCfg.ClientID = sp.config.KaConfig.KaClientId
	}

	sp.kaCtx, sp.kaCancel = context.WithCancel(context.Background())

	return &sp, nil
}

func (sp *kaScrapePool) run(t *Target) {
	//sp.mtx.RLock()
	//defer sp.mtx.RUnlock()
	level.Info(sp.logger).Log("msg", "kaScrapPool run start...")

	s := &targetScraper{Target: t, client: nil, timeout: 60 * time.Second}
	mrc := sp.config.MetricRelabelConfigs
	opts := scrapeLoopOptions{
		target:          t,
		scraper:         s,
		limit:           int(sp.config.SampleLimit),
		honorLabels:     sp.config.HonorLabels,
		honorTimestamps: sp.config.HonorTimestamps,
		mrc:             mrc,
	}

	// 复用scrapeLoop里的函数
	app := func() storage.Appender { return appender(sp.appendable.Appender(), opts.limit) }
	sm := func(l labels.Labels) labels.Labels {
		return mutateSampleLabels(l, opts.target, opts.honorLabels, opts.mrc)
	}
	rsm := func(l labels.Labels) labels.Labels {
		return mutateReportSampleLabels(l, opts.target)
	}
	sl := &scrapeLoop{
		scraper:             &targetScraper{Target: t},
		buffers:             nil,
		cache:               sp.ca,
		appender:            app,
		sampleMutator:       sm,
		reportSampleMutator: rsm,
		l:                   sp.logger,
		honorTimestamps:     sp.config.HonorTimestamps,
	}

LOOP:
	for {
		select {
		case msg := <-sp.message:
			if msg == "" {
				continue
			}
			if len(msg) > 3 && msg[0:4] == GZIP_V01 {
				b, err := util.UnGzip([]byte(msg[4:]))
				if err != nil {
					level.Error(sp.logger).Log("msg", "ungzip err", "err", err.Error())
				}
				msg = string(b)
			}

			start := time.Now()
			total, added, seriesAdded, appErr := sl.append([]byte(msg), "", start)
			if appErr != nil {
				level.Warn(sp.logger).Log("msg", "append failed", "err", appErr)
			}
			sp.report(total, added, seriesAdded, false)

		case <-sp.isStop:
			<-sp.kaCtx.Done()
			<-time.After(5 * time.Second)
			break LOOP
		}
	}

	level.Info(sp.logger).Log("msg", "kaScrapPool run exit...")
}

// stop stop the procedure
func (sp *kaScrapePool) stop() {
	sp.kaCancel()
	close(sp.isStop)
	sp.report(0, 0, 0, true)
	sp.wg.Wait()
}

// recv recv message from kafka
func (sp *kaScrapePool) recv(num int) {
	level.Info(sp.logger).Log("msg", "recv message start...", "num", num)
	client, err := sp.getKaConn()
	if err != nil {
		level.Error(sp.logger).Log("msg", "recv message get connect err", "num", num, "err", err.Error(), "topic", sp.config.KaConfig.KaTopic)
		panic("get kafka connect err")
	}
	level.Info(sp.logger).Log("msg", "recv message get connect success...", "num", num)

	consumer := newKaHandler(sp.message, sp.logger)
	for {
		level.Info(sp.logger).Log("msg", "recv message ready...", "num", num)
		err = client.Consume(sp.kaCtx, strings.Split(sp.config.KaConfig.KaTopic, ","), consumer)
		if err != nil {
			level.Error(sp.logger).Log("msg", "recv kafka message err", "num", num, "err", err.Error())
			panic("recv kafka message err")
		}
		// check if context was cancelled, signaling that the consumer should stop
		if sp.kaCtx.Err() != nil {
			level.Info(sp.logger).Log("msg", "recv kafka message is cancelled", "num", num)
			break
		}
	}
	client.Close()

	level.Info(sp.logger).Log("msg", "recv message end...", "num", num)
}

func (sp *kaScrapePool) loop(targets []*Target) {
	if !sp.GetChanged() {
		level.Info(sp.logger).Log("msg", "configuration has not changed")
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	start := func() {
		for {
			curNum := sp.getCurNum()
			level.Info(sp.logger).Log("curGoNum", curNum, "maxGoNum", sp.maxGoNums)
			if curNum >= sp.maxGoNums {
				break
			}

			sp.incCurNum()
			sp.wg.Add(1)
			go func() {
				defer sp.decCurNum()
				defer sp.wg.Done()
				sp.run(targets[0]) // targets 必须配置一个，方便复用之前的代码
			}()
		}
	}
	start()

	// recv message
	recvGoNum := sp.config.RecvGoNum
	if recvGoNum <= 0 {
		recvGoNum = 1
	}
	for i := 0; i < recvGoNum; i++ {
		go sp.recv(i + 1)
		<-time.After(300 * time.Millisecond)
	}

	for {
		select {
		case <-ticker.C:
			start()
		case <-sp.isReload:
			start()
		case <-sp.isStop:
			return
		}
	}
}

// Sync converts target groups into actual scrape targets and synchronizes
// the currently running scraper with the resulting set and returns all scraped and dropped targets.
func (sp *kaScrapePool) Sync(tgs []*targetgroup.Group) {
	// todo: droppedTargets 未处理

	var all []*Target
	for _, tg := range tgs {
		targets, err := targetsFromGroup(tg, sp.config)
		if err != nil {
			level.Error(sp.logger).Log("msg", "creating targets failed [kafka]", "err", err)
			continue
		}

		for _, t := range targets {
			if t.Labels().Len() > 0 {
				all = append(all, t)
			}
		}

	}

	go sp.loop(all)
}

// report save the count of scrape metric.
func (sp *kaScrapePool) report(totalCnt, addedCnt, seriesCnt int, isSave bool) {
	sp.lockCnt.Lock()
	defer sp.lockCnt.Unlock()

	sp.totalCnt += int64(totalCnt)
	sp.addedCnt += int64(addedCnt)
	sp.seriesCnt += int64(seriesCnt)
	curDate := time.Now().Format("2006010215")

	if isSave || sp.dateCnt != curDate {
		level.Info(sp.logger).Log("msg", "scrape records", "dateCnt", sp.dateCnt, "totalCnt", sp.totalCnt, "addedCnt", sp.addedCnt, "seriesCnt", sp.seriesCnt)
		sp.totalCnt = 0
		sp.addedCnt = 0
		sp.seriesCnt = 0
		sp.dateCnt = curDate
	}
}

// GetChanged return isChanged
func (sp *kaScrapePool) GetChanged() bool {
	sp.mtx.RLock()
	defer sp.mtx.RUnlock()

	return sp.isChange
}

// SetChanged set isChanged
func (sp *kaScrapePool) SetChanged(changed bool) {
	sp.mtx.Lock()
	defer sp.mtx.Unlock()

	sp.isChange = changed
}

// getKaConn return a kafka connect.
func (sp *kaScrapePool) getKaConn() (sarama.ConsumerGroup, error) {
	return sarama.NewConsumerGroup(strings.Split(sp.config.KaConfig.KaBroker, ","), sp.config.KaConfig.KaGroup, sp.kaCfg)
}

// getCurNum return current goroutine number
func (sp *kaScrapePool) getCurNum() int {
	sp.lockNum.Lock()
	defer sp.lockNum.Unlock()
	return sp.curGoNums
}

// incCurNum increase current goroutine number
func (sp *kaScrapePool) incCurNum() int {
	sp.lockNum.Lock()
	defer sp.lockNum.Unlock()
	sp.curGoNums++
	return sp.curGoNums
}

// decCurCnt decrement current goroutine number
func (sp *kaScrapePool) decCurNum() int {
	sp.lockNum.Lock()
	defer sp.lockNum.Unlock()
	sp.curGoNums--
	return sp.curGoNums
}

// kaHandler represents a Sarama consumer group consumer
type kaHandler struct {
	message chan string
	logger  log.Logger
}

// newKaHandler return a kaHandler object
func newKaHandler(msg chan string, logger log.Logger) *kaHandler {
	kh := kaHandler{
		message: msg,
		logger:  logger,
	}

	return &kh
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (kh *kaHandler) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	level.Info(kh.logger).Log("msg", "kaHandler Setup...")
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (kh *kaHandler) Cleanup(sarama.ConsumerGroupSession) error {
	level.Info(kh.logger).Log("msg", "kaHandler Cleanup...")
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (kh *kaHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	level.Info(kh.logger).Log("msg", "kaHandler ConsumeClaim start...")
	for message := range claim.Messages() {
		//fmt.Printf("Message claimed: value = %s, timestamp = %v, topic = %s\n", string(message.Value), message.Timestamp, message.Topic)
		session.MarkMessage(message, "")
		if len(message.Value) > 0 {
			kh.message <- string(message.Value)
		}
	}
	level.Info(kh.logger).Log("msg", "kaHandler ConsumeClaim end...")

	return nil
}
