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
	"github.com/prometheus/common/version"

	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/graphite"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/influxdb"
	"github.com/prometheus/prometheus/documentation/examples/remote_storage/remote_storage_adapter/opentsdb"
	"github.com/prometheus/prometheus/prompb"
)

type config struct {
	graphiteAddress         string
	graphiteTransport       string
	graphitePrefix          string
	opentsdbWURL            string
	opentsdbRURL            string
	influxdbURL             string
	influxdbRetentionPolicy string
	influxdbUsername        string
	influxdbDatabase        string
	influxdbPassword        string
	connTimeout             time.Duration
	remoteTimeout           time.Duration
	listenAddr              string
	telemetryPath           string
	promlogConfig           promlog.Config
	regular                 bool
	telnet                  bool
	maxConns                int
	maxIdle                 int
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
	gDateCnt = time.Now().Format("2006010215")
	gAllCnt  int64 // 总数
	gErrCnt  int64 // 失败数
)

func init() {
	prometheus.MustRegister(receivedSamples)
	prometheus.MustRegister(sentSamples)
	prometheus.MustRegister(failedSamples)
	prometheus.MustRegister(sentBatchDuration)

	prometheus.MustRegister(version.NewCollector("remote_storage_adapter"))
}

func main() {
	cfg := parseFlags()
	http.Handle(cfg.telemetryPath, promhttp.Handler())

	logger := promlog.New(&cfg.promlogConfig)

	level.Info(logger).Log("msg", "starting remote_storage_adapter", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	writers, readers := buildClients(logger, cfg)
	if err := serve(logger, cfg.listenAddr, writers, readers); err != nil {
		level.Error(logger).Log("msg", "Failed to listen", "addr", cfg.listenAddr, "err", err)
		report(logger, 0, 0, true)
		level.Info(logger).Log("msg", "See you next time!")
		os.Exit(1)
	}

	report(logger, 0, 0, true)
	for _, w := range writers {
		w.Destroy()
	}
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
	a.Flag("opentsdb-wurl", "The URL of the remote OpenTSDB server to write samples to. None, if empty.").
		Default("").StringVar(&cfg.opentsdbWURL)
	a.Flag("opentsdb-rurl", "The URL of the remote OpenTSDB server to read samples to. None, if empty.").
		Default("").StringVar(&cfg.opentsdbRURL)
	a.Flag("influxdb-url", "The URL of the remote InfluxDB server to send samples to. None, if empty.").
		Default("").StringVar(&cfg.influxdbURL)
	a.Flag("influxdb.retention-policy", "The InfluxDB retention policy to use.").
		Default("autogen").StringVar(&cfg.influxdbRetentionPolicy)
	a.Flag("influxdb.username", "The username to use when sending samples to InfluxDB. The corresponding password must be provided via the INFLUXDB_PW environment variable.").
		Default("").StringVar(&cfg.influxdbUsername)
	a.Flag("influxdb.database", "The name of the database to use for storing samples in InfluxDB.").
		Default("prometheus").StringVar(&cfg.influxdbDatabase)
	a.Flag("conn-timeout", "The timeout to use when connecting to the remote storage.(default: 3s)").
		Default("3s").DurationVar(&cfg.connTimeout)
	a.Flag("send-timeout", "The timeout to use when sending samples to the remote storage.").
		Default("30s").DurationVar(&cfg.remoteTimeout)
	a.Flag("web.listen-address", "Address to listen on for web endpoints.").
		Default(":9201").StringVar(&cfg.listenAddr)
	a.Flag("web.telemetry-path", "Address to listen on for web endpoints.").
		Default("/metrics").StringVar(&cfg.telemetryPath)
	a.Flag("regular", "The program support regular expression.").
		BoolVar(&cfg.regular)
	a.Flag("telnet", "The program support regular expression.").
		BoolVar(&cfg.telnet)
	a.Flag("max-conns", "The maximum connection.(default: 200)").
		Default("200").IntVar(&cfg.maxConns)
	a.Flag("max-idle", "The maximum number of idle connections.(default: 200)").
		Default("200").IntVar(&cfg.maxIdle)

	flag.AddFlags(a, &cfg.promlogConfig)
	a.Version(version.Print("Remote storage adapter"))

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
	Destroy()
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
	if cfg.opentsdbWURL != "" || cfg.opentsdbRURL != "" {
		c := opentsdb.NewClient(
			log.With(logger, "storage", "OpenTSDB"),
			cfg.opentsdbWURL,
			cfg.opentsdbRURL,
			0,
			cfg.connTimeout,
			cfg.remoteTimeout,
			cfg.regular,
			cfg.telnet,
			false,
			cfg.maxConns,
			cfg.maxIdle,
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
	addr    string
	writers []writer
	readers []reader
	logger  log.Logger
}

func newRemoteHandler(logger log.Logger, addr string, writers []writer, readers []reader) *RemoteHandler {
	return &RemoteHandler{
		addr:    addr,
		writers: writers,
		readers: readers,
		logger:  logger,
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
				sendSamples(h.logger, rw, samples)
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

func serve(logger log.Logger, addr string, writers []writer, readers []reader) error {
	rHandler := newRemoteHandler(logger, addr, writers, readers)
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

func sendSamples(logger log.Logger, w writer, samples model.Samples) {
	begin := time.Now()
	errCnt, err := w.Write(samples)
	duration := time.Since(begin).Seconds()
	report(logger, len(samples), errCnt, false)
	if err != nil {
		level.Warn(logger).Log("msg", "Error sending samples to remote storage", "err", err, "storage", w.Name(), "num_samples", len(samples))
		failedSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	}
	sentSamples.WithLabelValues(w.Name()).Add(float64(len(samples)))
	sentBatchDuration.WithLabelValues(w.Name()).Observe(duration)
}

// report save the count of scrape metric.
func report(logger log.Logger, allCnt, errCnt int, isSave bool) {
	gLockCnt.Lock()
	defer gLockCnt.Unlock()

	gAllCnt += int64(allCnt)
	gErrCnt += int64(errCnt)
	curDate := time.Now().Format("2006010215")

	if isSave || gDateCnt != curDate {
		level.Info(logger).Log("msg", "storage records", "dateCnt", gDateCnt, "totalCnt", gAllCnt, "sucCnt", gAllCnt-gErrCnt, "errCnt", gErrCnt)
		gAllCnt = 0
		gErrCnt = 0
		gDateCnt = curDate
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
