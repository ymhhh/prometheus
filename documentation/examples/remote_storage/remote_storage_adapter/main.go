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

// The main package for the Prometheus server executable.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"gopkg.in/alecthomas/kingpin.v2"

	influx "github.com/influxdata/influxdb/client/v2"

	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/graphite"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/influxdb"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/opentsdb"
	"github.com/prometheus/prometheus/prompb"
)

type config struct {
	graphiteAddress         string
	graphiteTransport       string
	graphitePrefix          string
	opentsdbURL             string
	influxdbURL             string
	influxdbRetentionPolicy string
	influxdbUsername        string
	influxdbDatabase        string
	influxdbPassword        string
	remoteTimeout           time.Duration
	listenAddr              string
	telemetryPath           string
	promlogConfig           promlog.Config
	maxRecNum               int
}

var (
	receivedSamples = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "received_samples_total",
			Help: "Total number of received samples.",
		},
	)
	sentSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sent_samples_total",
			Help: "Total number of processed samples sent to remote storage.",
		},
		[]string{"remote"},
	)
	failedSamples = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_samples_total",
			Help: "Total number of processed samples which failed on send to remote storage.",
		},
		[]string{"remote"},
	)
	sentBatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sent_batch_duration_seconds",
			Help:    "Duration of sample batch send calls to the remote storage.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"remote"},
	)

	gLockCnt sync.Mutex
	gAllCnt  int // 总数
	gSucCnt  int // 成功数
)

func init() {
	prometheus.MustRegister(receivedSamples)
	prometheus.MustRegister(sentSamples)
	prometheus.MustRegister(failedSamples)
	prometheus.MustRegister(sentBatchDuration)
}

func main() {
	cfg := parseFlags()
	http.Handle(cfg.telemetryPath, promhttp.Handler())

	logger := promlog.New(&cfg.promlogConfig)

	writers, readers := buildClients(logger, cfg)
	if err := serve(logger, cfg.listenAddr, cfg.maxRecNum, writers, readers); err != nil {
		level.Error(logger).Log("msg", "Failed to listen", "addr", cfg.listenAddr, "err", err)
	}

	report(logger, 0, 0, cfg.maxRecNum, true)
	level.Info(logger).Log("msg", "See you next time!")
}

func parseFlags() *config {
	a := kingpin.New(filepath.Base(os.Args[0]), "Remote storage adapter")
	a.HelpFlag.Short('h')

	cfg := &config{
		influxdbPassword: os.Getenv("INFLUXDB_PW"),
		promlogConfig:    promlog.Config{},
	}

	a.Flag("graphite-address", "The host:port of the Graphite server to send samples to. None, if empty.").
		Default("").StringVar(&cfg.graphiteAddress)
	a.Flag("graphite-transport", "Transport protocol to use to communicate with Graphite. 'tcp', if empty.").
		Default("tcp").StringVar(&cfg.graphiteTransport)
	a.Flag("graphite-prefix", "The prefix to prepend to all metrics exported to Graphite. None, if empty.").
		Default("").StringVar(&cfg.graphitePrefix)
	a.Flag("opentsdb-url", "The URL of the remote OpenTSDB server to send samples to. None, if empty.").
		Default("").StringVar(&cfg.opentsdbURL)
	a.Flag("influxdb-url", "The URL of the remote InfluxDB server to send samples to. None, if empty.").
		Default("").StringVar(&cfg.influxdbURL)
	a.Flag("influxdb.retention-policy", "The InfluxDB retention policy to use.").
		Default("autogen").StringVar(&cfg.influxdbRetentionPolicy)
	a.Flag("influxdb.username", "The username to use when sending samples to InfluxDB. The corresponding password must be provided via the INFLUXDB_PW environment variable.").
		Default("").StringVar(&cfg.influxdbUsername)
	a.Flag("influxdb.database", "The name of the database to use for storing samples in InfluxDB.").
		Default("prometheus").StringVar(&cfg.influxdbDatabase)
	a.Flag("send-timeout", "The timeout to use when sending samples to the remote storage.").
		Default("30s").DurationVar(&cfg.remoteTimeout)
	a.Flag("web.listen-address", "Address to listen on for web endpoints.").
		Default(":9201").StringVar(&cfg.listenAddr)
	a.Flag("web.telemetry-path", "Address to listen on for web endpoints.").
		Default("/metrics").StringVar(&cfg.telemetryPath)
	a.Flag("max_rec_num", "Record the maximum value of statistical log.").
		Default("0").IntVar(&cfg.maxRecNum)

	flag.AddFlags(a, &cfg.promlogConfig)

	_, err := a.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing commandline arguments"))
		a.Usage(os.Args[1:])
		os.Exit(2)
	}

	return cfg
}

type writer interface {
	Write(samples model.Samples) (int, error)
	Name() string
}

type reader interface {
	Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error)
	Name() string
}

func buildClients(logger log.Logger, cfg *config) ([]writer, []reader) {
	var writers []writer
	var readers []reader
	if cfg.graphiteAddress != "" {
		c := graphite.NewClient(
			log.With(logger, "storage", "Graphite"),
			cfg.graphiteAddress, cfg.graphiteTransport,
			cfg.remoteTimeout, cfg.graphitePrefix)
		writers = append(writers, c)
	}
	if cfg.opentsdbURL != "" {
		c := opentsdb.NewClient(
			log.With(logger, "storage", "OpenTSDB"),
			cfg.opentsdbURL,
			cfg.remoteTimeout,
		)
		writers = append(writers, c)
		readers = append(readers, c)
	}
	if cfg.influxdbURL != "" {
		url, err := url.Parse(cfg.influxdbURL)
		if err != nil {
			level.Error(logger).Log("msg", "Failed to parse InfluxDB URL", "url", cfg.influxdbURL, "err", err)
			os.Exit(1)
		}
		conf := influx.HTTPConfig{
			Addr:     url.String(),
			Username: cfg.influxdbUsername,
			Password: cfg.influxdbPassword,
			Timeout:  cfg.remoteTimeout,
		}
		c := influxdb.NewClient(
			log.With(logger, "storage", "InfluxDB"),
			conf,
			cfg.influxdbDatabase,
			cfg.influxdbRetentionPolicy,
		)
		prometheus.MustRegister(c)
		writers = append(writers, c)
		readers = append(readers, c)
	}
	level.Info(logger).Log("msg", "Starting up...", "addr", cfg.listenAddr)
	return writers, readers
}

// RemoteHandler handler
type RemoteHandler struct {
	addr      string
	maxRecNum int
	writers   []writer
	readers   []reader
	logger    log.Logger
}

func newRemoteHandler(logger log.Logger, addr string, maxRecNum int, writers []writer, readers []reader) *RemoteHandler {
	return &RemoteHandler{
		addr:      addr,
		maxRecNum: maxRecNum,
		writers:   writers,
		readers:   readers,
		logger:    logger,
	}
}

func (h *RemoteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/write" {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			level.Error(h.logger).Log("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			level.Error(h.logger).Log("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.WriteRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			level.Error(h.logger).Log("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		samples := protoToSamples(&req)
		receivedSamples.Add(float64(len(samples)))

		var wg sync.WaitGroup
		for _, w := range h.writers {
			wg.Add(1)
			go func(rw writer) {
				sendSamples(h.logger, rw, samples, h.maxRecNum)
				wg.Done()
			}(w)
		}
		wg.Wait()
	} else if r.URL.Path == "/read" {
		compressed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			level.Error(h.logger).Log("msg", "Read error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqBuf, err := snappy.Decode(nil, compressed)
		if err != nil {
			level.Error(h.logger).Log("msg", "Decode error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req prompb.ReadRequest
		if err := proto.Unmarshal(reqBuf, &req); err != nil {
			level.Error(h.logger).Log("msg", "Unmarshal error", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO: Support reading from more than one reader and merging the results.
		if len(h.readers) != 1 {
			http.Error(w, fmt.Sprintf("expected exactly one reader, found %d readers", len(h.readers)), http.StatusInternalServerError)
			return
		}
		reader := h.readers[0]

		var resp *prompb.ReadResponse
		resp, err = reader.Read(&req)
		if err != nil {
			level.Warn(h.logger).Log("msg", "Error executing query", "query", req, "storage", reader.Name(), "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := proto.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Content-Encoding", "snappy")

		compressed = snappy.Encode(nil, data)
		if _, err := w.Write(compressed); err != nil {
			level.Warn(h.logger).Log("msg", "Error writing response", "storage", reader.Name(), "err", err)
		}
	}
}

func serve(logger log.Logger, addr string, maxRecNum int, writers []writer, readers []reader) error {
	rHandler := newRemoteHandler(logger, addr, maxRecNum, writers, readers)
	srv := &http.Server{
		Addr:         addr,
		Handler:      rHandler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	go interruptHandler(srv, logger)
	return srv.ListenAndServe()
}

func protoToSamples(req *prompb.WriteRequest) model.Samples {
	var samples model.Samples
	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			metric[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}

		for _, s := range ts.Samples {
			samples = append(samples, &model.Sample{
				Metric:    metric,
				Value:     model.SampleValue(s.Value),
				Timestamp: model.Time(s.Timestamp),
			})
		}
	}
	return samples
}

func sendSamples(logger log.Logger, w writer, samples model.Samples, maxRecNum int) {
	begin := time.Now()
	sucCnt, err := w.Write(samples)
	duration := time.Since(begin).Seconds()
	report(logger, len(samples), sucCnt, maxRecNum, false)
	if err != nil {
		level.Warn(logger).Log("msg", "Error sending samples to remote storage", "err", err, "storage", w.Name(), "num_samples", len(samples))
		failedSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	}
	sentSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	sentBatchDuration.WithLabelValues(w.Name()).Observe(duration)
}

// report save the count of scrape metric.
func report(logger log.Logger, allCnt, sucCnt, maxRecNum int, isSave bool) {
	if maxRecNum == 0 {
		return
	}

	gLockCnt.Lock()
	defer gLockCnt.Unlock()

	gAllCnt += allCnt
	gSucCnt += sucCnt

	if isSave || gAllCnt >= maxRecNum {
		level.Info(logger).Log("msg", "storage records", "totalCnt", gAllCnt, "sucCnt", gSucCnt, "errCnt", gAllCnt-gSucCnt)
		gAllCnt = 0
		gSucCnt = 0
	}
}

// interruptHandler capture signal
func interruptHandler(srv *http.Server, logger log.Logger) {
	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, os.Interrupt, syscall.SIGTERM)
	<-notifier
	level.Info(logger).Log("msg", "received SIGINT/SIGTERM; exiting gracefully...")

	ctx, _ := context.WithTimeout(context.Background(), 10)
	srv.Shutdown(ctx)
}
