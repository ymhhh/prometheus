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
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util"
	"github.com/xdg/scram"
	"hash"
	"strings"
	"sync"
	"time"
)

var (
	SHA256       scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
	MSG_CHAN_LEN                        = 500
)

const (
	MSG_T_JSON = "json"
	MSG_T_PROM = "prom"
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
	appendable Appendable
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
}

func newKaScrapePool(cfg *config.ScrapeConfig, app Appendable, jitterSeed uint64, logger log.Logger) (*kaScrapePool, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	msgLen := MSG_CHAN_LEN
	if msgLen > cfg.MaxGoNum {
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
	}

	if cfg.MsgType == "MSG_T_JSON" && len(cfg.MsgLabel) == 0 {
		return &sp, errors.New("msg label is empty")
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

	sp.kaCtx, sp.kaCancel = context.WithCancel(context.Background())

	return &sp, nil
}

func (sp *kaScrapePool) run(t *Target) {
	//sp.mtx.RLock()
	//defer sp.mtx.RUnlock()
	level.Info(sp.logger).Log("msg", "kaScrapPool run start...")

	ca := newScrapeCache()
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
	app := func() storage.Appender {
		ad, err := sp.appendable.Appender()
		if err != nil {
			panic(err)
		}
		return appender(ad, 0)
	}
	sm := func(l labels.Labels) labels.Labels {
		return mutateSampleLabels(l, opts.target, opts.honorLabels, opts.mrc)
	}
	rsm := func(l labels.Labels) labels.Labels {
		return mutateReportSampleLabels(l, opts.target)
	}
	sl := &scrapeLoop{
		scraper:             &targetScraper{Target: t},
		buffers:             nil,
		cache:               ca,
		appender:            app,
		sampleMutator:       sm,
		reportSampleMutator: rsm,
		l:                   sp.logger,
		honorTimestamps:     sp.config.HonorTimestamps,
	}

	var err error
LOOP:
	for {
		select {
		case msg := <-sp.message:
			if msg == "" {
				continue
			}
			if sp.config.MsgType == MSG_T_JSON {
				msg, err = sp.json2Prome(&msg)
				if err != nil {
					level.Error(sp.logger).Log("msg", "json to prome err", "err", err.Error())
					continue
				}
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
func (sp *kaScrapePool) recv() {
	level.Info(sp.logger).Log("msg", "recv message start...")
	client, err := sp.getKaConn()
	if err != nil {
		level.Error(sp.logger).Log("msg", "recv message get connect err", "err", err.Error(), "topic", sp.config.KaConfig.KaTopic)
		panic("get kafka connect err")
	}
	level.Info(sp.logger).Log("msg", "recv message get connect success...")

	consumer := newKaHandler(sp.message, sp.logger)
	for {
		level.Info(sp.logger).Log("msg", "recv message ready...")
		err = client.Consume(sp.kaCtx, strings.Split(sp.config.KaConfig.KaTopic, ","), consumer)
		if err != nil {
			level.Error(sp.logger).Log("msg", "recv kafka message err", "err", err.Error())
			panic("recv kafka message err")
		}
		// check if context was cancelled, signaling that the consumer should stop
		if sp.kaCtx.Err() != nil {
			level.Info(sp.logger).Log("msg", "recv kafka message is cancelled")
			break
		}
	}
	client.Close()

	level.Info(sp.logger).Log("msg", "recv message end...")
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
	go sp.recv()

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

// json2Prome json转prome格式
func (sp *kaScrapePool) json2Prome(msg *string) (string, error) {
	md := MetricData{}
	err := json.Unmarshal([]byte(*msg), &md)
	if err != nil {
		return "", err
	}

	// label
	var buf bytes.Buffer
	for _, k := range sp.config.MsgLabel {
		if label, ok := md[k].(string); ok {
			buf.WriteString(k + `="` + label + `",`)
		} else {
			buf.WriteString(k + `="",`)
		}
		delete(md, k)
	}
	labels := buf.String()

	// value
	have := false
	buf.Reset()
	for k, v := range md {
		// value, "val":200
		if t, ok := v.(float64); ok {
			if !util.CheckMetircName(k) {
				level.Error(sp.logger).Log("msg", "metric name fmt err", "name", k)
				continue
			}
			buf.WriteString("# TYPE " + k + " untyped\n")
			buf.WriteString(k)
			buf.WriteString("{")
			buf.WriteString(labels)
			buf.WriteString("} ")
			buf.WriteString(fmt.Sprintf("%f\n", t))
			have = true
			continue
		}

		// key-value map, "val":{"v1": 100, "v2":200}
		if t, ok := v.(map[string]interface{}); ok {
			for k1, v1 := range t {
				if _, ok := v1.(float64); !ok {
					continue
				}
				name := k + "_" + k1
				if !util.CheckMetircName(name) {
					level.Error(sp.logger).Log("msg", "metric name fmt err", "name", name)
					continue
				}
				buf.WriteString("# TYPE " + name + " untyped\n")
				buf.WriteString(name)
				buf.WriteString("{")
				buf.WriteString(labels)
				buf.WriteString("} ")
				buf.WriteString(fmt.Sprintf("%f\n", v1))
				have = true
			}
			continue
		}

		// key-value list, "val":[{"v11": 100, "v12":200}, {"v21":300, "v22":400}]
		if t, ok := v.([]interface{}); ok {
			m := map[string]float64{}
			for _, t1 := range t {
				if v1, ok := t1.(map[string]interface{}); ok {
					for k2, v2 := range v1 {
						if t2, ok := v2.(float64); ok {
							m[k+"_"+k2] = t2
						}
					} // end for k2
				}
			} // end for _, t1

			for k1, v1 := range m {
				if !util.CheckMetircName(k1) {
					level.Error(sp.logger).Log("msg", "metric name fmt err", "name", k1)
					continue
				}
				buf.WriteString("# TYPE " + k1 + " untyped\n")
				buf.WriteString(k1)
				buf.WriteString("{")
				buf.WriteString(labels)
				buf.WriteString("} ")
				buf.WriteString(fmt.Sprintf("%f\n", v1))
				have = true
			}
		}
	}

	if !have {
		return "", errors.New("no metric data")
	}

	return buf.String(), nil
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
		level.Info(sp.logger).Log("msg", "scrapt records", "totalCnt", sp.totalCnt, "addedCnt", sp.addedCnt, "seriesCnt", sp.seriesCnt)
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
