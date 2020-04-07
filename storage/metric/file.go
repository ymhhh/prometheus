// Copyright 2020 JD BDP <huanghonghu@jd.com>
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

//metric 将metric以及对应的label存入文件

package metric

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/go-kit/kit/log"
	"github.com/go-trellis/common/logger"
	"github.com/go-trellis/config"
	"github.com/pkg/errors"
)

type ReadyStorage struct {
	cfg config.Config
	w   logger.Writer

	l log.Logger

	appender storage.Appender
	querier  storage.Querier
}

type Appender struct {
	l log.Logger

	fLog logger.Logger
}

type Querier struct{}

// NewStorage 生成对象
func NewStorage(configFile string, l log.Logger) (storage.Storage, error) {
	cfg, err := config.NewConfig(configFile)
	if err != nil {
		return nil, err
	}

	fileName := cfg.GetString("metric_storage.filename")
	if len(fileName) == 0 {
		return nil, fmt.Errorf("file name not set")
	}

	fLog := logger.NewLogger()

	w, err := logger.FileWriter(fLog,
		logger.FileWiterFileName(fileName),
		logger.FileWiterBuffer(cfg.GetInt("metric_storage.chan_buffer", 100000)),
		logger.FileWiterSeparator(cfg.GetString("metric_storage.separator", "|")),
		logger.FileWiterLevel(logger.Level(cfg.GetInt("metric_storage.level", 2))),
		logger.FileWiterMoveFileType(logger.MoveFileType(cfg.GetInt("metric_storage.move_file_type", 2))),
		logger.FileWiterMaxLength(int64(cfg.GetInt("metric_storage.max_length", 10000000))),
		logger.FileWiterRoutings(cfg.GetInt("metric_storage.routings")),
	)
	if err != nil {
		return nil, err
	}

	s := &ReadyStorage{
		cfg:     cfg,
		l:       l,
		querier: &Querier{},
	}

	appender := &Appender{l: l, fLog: fLog}
	s.w = w

	_, err = appender.fLog.Subscriber(w)
	if err != nil {
		return nil, err
	}

	s.appender = appender
	return s, nil
}

// Appender returns a new appender against the storage.
func (p *ReadyStorage) Appender() (storage.Appender, error) {
	return p.appender, nil
}

// Close closes the storage and all its underlying resources.
func (p *ReadyStorage) Close() error {
	p.w.Stop()
	return nil
}

func (p *ReadyStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return p.querier, nil
}

// StartTime returns the oldest timestamp stored in the storage.
func (p *ReadyStorage) StartTime() (int64, error) {
	return 0, nil
}

func (p *Appender) Add(lsets labels.Labels, t int64, v float64) (uint64, error) {

	metric, serviceID := "", "normal"
	var lns []string
	for _, lset := range lsets {
		switch lset.Name {
		case labels.MetricName:
			metric = lset.Value
		case "serviceId":
			serviceID = lset.Value
		default:
			lns = append(lns, lset.Name)
		}
	}

	if len(metric) == 0 {
		return 0, errors.New("not found metric")
	}
	p.write(serviceID, metric, lns)

	return uint64(len(lns)), nil
}

func (p *Appender) AddFast(lsets labels.Labels, ref uint64, t int64, v float64) error {
	metric, serviceID := "", "normal"
	var lns []string
	for _, lset := range lsets {
		switch lset.Name {
		case labels.MetricName:
			metric = lset.Value
		case "serviceId":
			serviceID = lset.Value
		default:
			lns = append(lns, lset.Name)
		}
	}

	if len(metric) == 0 {
		return errors.New("not found metric")
	}
	p.write(serviceID, metric, lns)

	return nil
}

func (p *Appender) write(service, metric string, labels []string) {
	p.fLog.Info(service, metric, strings.Join(labels, ","))
}

// Commit submits the collected samples and purges the batch.
func (p *Appender) Commit() error {
	return nil
}

// Rollback rollback the collected samples and purges the batch.
func (p *Appender) Rollback() error {
	return nil
}

// Close releases the resources of the Querier.
func (p *Querier) Close() error {
	return nil
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (p *Querier) LabelNames() ([]string, storage.Warnings, error) {
	return nil, nil, nil
}

// LabelValues returns all potential values for a label name.
func (p *Querier) LabelValues(name string) ([]string, storage.Warnings, error) {
	return nil, nil, nil
}

// Select returns a set of series that matches the given label matchers.
func (p *Querier) Select(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	return &seriesSet{}, nil, nil
}

// SelectSorted returns a sorted set of series that matches the given label matchers.
func (p *Querier) SelectSorted(*storage.SelectParams, ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	return &seriesSet{}, nil, nil
}

type seriesSet struct {
}

func (s seriesSet) Next() bool         { return false }
func (s seriesSet) Err() error         { return nil }
func (s seriesSet) At() storage.Series { return nil }
