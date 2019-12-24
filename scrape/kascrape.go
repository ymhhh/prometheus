package scrape

import (
	"context"
	"crypto/sha256"
	"github.com/Shopify/sarama"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/pkg/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/xdg/scram"
	"hash"
	"io"
	"math"
	"strings"
	"sync"
	"time"
)

var (
	nowFunc                        = time.Now
	SHA256  scram.HashGeneratorFcn = func() hash.Hash { return sha256.New() }
)

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

	mtx    sync.RWMutex
	config *config.ScrapeConfig
	//client *http.Client
	// Targets and loops must always be synchronized to have the same
	// set of hashes.
	//activeTargets  map[uint64]*Target
	droppedTargets []*Target
	//loops          map[uint64]loop
	//cancel         context.CancelFunc
	//
	//// Constructor for new scrape loops. This is settable for testing convenience.
	//newLoop func(scrapeLoopOptions) loop

	maxGoNums int        // max goroutine number
	curGoNums int        // current goroutine number
	lockNum   sync.Mutex // lock for goroutine
	wg        sync.WaitGroup
	isStop    chan bool
	isReload  chan bool

	kaCfg    *sarama.Config
	kaCtx    context.Context
	kaCancel context.CancelFunc
	message  chan string
}

func newKaScrapePool(cfg *config.ScrapeConfig, app Appendable, jitterSeed uint64, logger log.Logger) (*kaScrapePool, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	sp := kaScrapePool{
		config:     cfg,
		appendable: app,
		maxGoNums:  cfg.MaxGoNum,
		curGoNums:  0,
		isStop:     make(chan bool),
		isReload:   make(chan bool),
		message:    make(chan string),
		logger:     logger,
	}

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
	level.Info(sp.logger).Log("msg", "kaScrapPool run start...", "job_name", sp.config.JobName)

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
	sm := func(l labels.Labels) labels.Labels {
		return mutateSampleLabels(l, opts.target, opts.honorLabels, opts.mrc)
	}

	for {
		select {
		case msg := <-sp.message:
			start := time.Now()
			_, _, _, appErr := sp.append([]byte(msg), "", start, ca, sm)
			if appErr != nil {
				level.Warn(sp.logger).Log("msg", "append failed", "err", appErr)
				// The append failed, probably due to a parse error or sample limit.
				// Call sl.append again with an empty scrape to trigger stale markers.
				//if _, _, _, err := kh.append(message.Value, "", start); err != nil {
				//	level.Warn(kh.logger).Log("msg", "append failed", "err", err)
				//}
			}
		case <-sp.isStop:
			goto END

		}
	}

END:
	level.Info(sp.logger).Log("msg", "kaScrapPool run exit...", "job_name", sp.config.JobName)
}

// stop stop the procedure
func (sp *kaScrapePool) stop() {
	sp.kaCancel()
	close(sp.isStop)
	sp.wg.Wait()
}

// recv recv message from kafka
func (sp *kaScrapePool) recv() {
	level.Info(sp.logger).Log("msg", "recv message start...")
	level.Info(sp.logger).Log("msg", "recv message get connect start...")
	client, err := sp.getKaConn()
	if err != nil {
		level.Error(sp.logger).Log("msg", "recv message get connect err", "err", err.Error())
		panic("get kafka connect err")
	}
	level.Info(sp.logger).Log("msg", "recv message get connect success...")

	consumer := newKaHandler(sp.message, sp.logger)
	for {
		level.Info(sp.logger).Log("msg", "recv message ready...", "job_name", sp.config.JobName)
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

	level.Info(sp.logger).Log("msg", "recv message end...")
}

func (sp *kaScrapePool) loop(targets []*Target) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	start := func() {
		for {
			if sp.getCurNum() >= sp.maxGoNums {
				break
			}
			level.Info(sp.logger).Log("job_name", sp.config.JobName, "curGoNum", sp.getCurNum(), "maxGoNum", sp.maxGoNums)

			sp.incCurNum()
			go func() {
				sp.wg.Add(1)
				defer sp.decCurNum()
				defer sp.wg.Done()
				sp.run(targets[0]) // targets 必须配置一个，方便复用之前的代码
			}()
		}
	}
	start()

	// recv message
	go func() {
		sp.wg.Add(1)
		defer sp.wg.Done()
		sp.recv()
	}()

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

// append add message to TSDB
func (sp *kaScrapePool) append(b []byte, contentType string, ts time.Time, ca *scrapeCache, sm func(l labels.Labels) labels.Labels) (total, added, seriesAdded int, err error) {
	var (
		//app            = sl.appender()
		app = func() storage.Appender {
			ad, err := sp.appendable.Appender()
			if err != nil {
				panic(err)
			}
			return appender(ad, 0)
		}()
		p              = textparse.New(b, contentType)
		defTime        = timestamp.FromTime(ts)
		numOutOfOrder  = 0
		numDuplicates  = 0
		numOutOfBounds = 0
	)

	var sampleLimitErr error

loop:
	for {
		var et textparse.Entry
		if et, err = p.Next(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		switch et {
		case textparse.EntryType:
			ca.setType(p.Type())
			continue
		case textparse.EntryHelp:
			ca.setHelp(p.Help())
			continue
		case textparse.EntryUnit:
			ca.setUnit(p.Unit())
			continue
		case textparse.EntryComment:
			continue
		default:
		}
		total++

		t := defTime
		met, tp, v := p.Series()
		if !sp.config.HonorTimestamps {
			tp = nil
		}
		if tp != nil {
			t = *tp
		}

		if ca.getDropped(yoloString(met)) {
			continue
		}
		ce, ok := ca.get(yoloString(met))
		if ok {
			switch err = app.AddFast(ce.lset, ce.ref, t, v); err {
			case nil:
				if tp == nil {
					ca.trackStaleness(ce.hash, ce.lset)
				}
			case storage.ErrNotFound:
				ok = false
			case storage.ErrOutOfOrderSample:
				numOutOfOrder++
				level.Debug(sp.logger).Log("msg", "Out of order sample", "series", string(met))
				targetScrapeSampleOutOfOrder.Inc()
				continue
			case storage.ErrDuplicateSampleForTimestamp:
				numDuplicates++
				level.Debug(sp.logger).Log("msg", "Duplicate sample for timestamp", "series", string(met))
				targetScrapeSampleDuplicate.Inc()
				continue
			case storage.ErrOutOfBounds:
				numOutOfBounds++
				level.Debug(sp.logger).Log("msg", "Out of bounds metric", "series", string(met))
				targetScrapeSampleOutOfBounds.Inc()
				continue
			case errSampleLimit:
				// Keep on parsing output if we hit the limit, so we report the correct
				// total number of samples scraped.
				sampleLimitErr = err
				added++
				continue
			default:
				break loop
			}
		}
		if !ok {
			var lset labels.Labels

			mets := p.Metric(&lset)
			hash := lset.Hash()

			// Hash label set as it is seen local to the target. Then add target labels
			// and relabeling and store the final label set.
			lset = sm(lset)

			// The label set may be set to nil to indicate dropping.
			if lset == nil {
				ca.addDropped(mets)
				continue
			}

			var ref uint64
			ref, err = app.Add(lset, t, v)
			switch err {
			case nil:
			case storage.ErrOutOfOrderSample:
				err = nil
				numOutOfOrder++
				level.Debug(sp.logger).Log("msg", "Out of order sample", "series", string(met))
				targetScrapeSampleOutOfOrder.Inc()
				continue
			case storage.ErrDuplicateSampleForTimestamp:
				err = nil
				numDuplicates++
				level.Debug(sp.logger).Log("msg", "Duplicate sample for timestamp", "series", string(met))
				targetScrapeSampleDuplicate.Inc()
				continue
			case storage.ErrOutOfBounds:
				err = nil
				numOutOfBounds++
				level.Debug(sp.logger).Log("msg", "Out of bounds metric", "series", string(met))
				targetScrapeSampleOutOfBounds.Inc()
				continue
			case errSampleLimit:
				sampleLimitErr = err
				added++
				continue
			default:
				level.Debug(sp.logger).Log("msg", "unexpected error", "series", string(met), "err", err)
				break loop
			}
			if tp == nil {
				// Bypass staleness logic if there is an explicit timestamp.
				ca.trackStaleness(hash, lset)
			}
			ca.addRef(mets, ref, lset, hash)
			seriesAdded++
		}
		added++
	}
	if sampleLimitErr != nil {
		if err == nil {
			err = sampleLimitErr
		}
		// We only want to increment this once per scrape, so this is Inc'd outside the loop.
		targetScrapeSampleLimit.Inc()
	}
	if numOutOfOrder > 0 {
		level.Warn(sp.logger).Log("msg", "Error on ingesting out-of-order samples", "num_dropped", numOutOfOrder)
	}
	if numDuplicates > 0 {
		level.Warn(sp.logger).Log("msg", "Error on ingesting samples with different value but same timestamp", "num_dropped", numDuplicates)
	}
	if numOutOfBounds > 0 {
		level.Warn(sp.logger).Log("msg", "Error on ingesting samples that are too old or are too far into the future", "num_dropped", numOutOfBounds)
	}
	if err == nil {
		ca.forEachStale(func(lset labels.Labels) bool {
			// Series no longer exposed, mark it stale.
			_, err = app.Add(lset, defTime, math.Float64frombits(value.StaleNaN))
			switch err {
			case storage.ErrOutOfOrderSample, storage.ErrDuplicateSampleForTimestamp:
				// Do not count these in logging, as this is expected if a target
				// goes away and comes back again with a new scrape loop.
				err = nil
			}
			return err == nil
		})
	}
	if err != nil {
		app.Rollback()
		return total, added, seriesAdded, err
	}
	if err := app.Commit(); err != nil {
		return total, added, seriesAdded, err
	}

	// Only perform cache cleaning if the scrape was not empty.
	// An empty scrape (usually) is used to indicate a failed scrape.
	ca.iterDone(len(b) > 0)

	return total, added, seriesAdded, nil
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
