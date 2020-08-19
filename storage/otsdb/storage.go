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
	"context"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"

	"github.com/go-trellis/config"
)

var (
	defOtsdbBuffLen = 1000
	maxOtsdbBuffLen = 200000
	gOtsdbBuff      chan *model.Sample
)

// Querier implements storage.Storage.
type ReadyStorage struct {
	appender storage.Appender
	querier  storage.Querier
}

// NewReadyStorage 实例化ReadyStorage
func NewReadyStorage(configFile string) storage.Storage {
	buffLen := defOtsdbBuffLen
	cfg, err := config.NewConfig(configFile)
	if err == nil {
		batchNum := cfg.GetInt("otsdb_configs.batch_num")
		recvGoNum := cfg.GetInt("otsdb_configs.recv_go_num")
		buffLen = batchNum * recvGoNum
		if buffLen <= 0 {
			buffLen = defOtsdbBuffLen
		} else if buffLen > maxOtsdbBuffLen {
			buffLen = maxOtsdbBuffLen
		}
	}

	gOtsdbBuff = make(chan *model.Sample, buffLen)
	app := newAppender()

	s := &ReadyStorage{
		appender: app,
		querier:  &Querier{},
	}

	return s
}

// StartTime returns the oldest timestamp stored in the storage.
func (s *ReadyStorage) StartTime() (int64, error) {
	return int64(model.Latest), nil
}

// Close closes the storage and all its underlying resources.
func (s *ReadyStorage) Close() error {
	if gOtsdbBuff != nil {
		close(gOtsdbBuff)
	}

	return nil
}

// Querier returns a new Querier on the storage.
func (p *ReadyStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return p.querier, nil
}

// Appender returns a new appender against the storage.
func (p *ReadyStorage) Appender() storage.Appender {
	return p.appender
}

// Querier implements storage.Querier.
type Querier struct{}

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
// Caller can specify if it requires returned series to be sorted. Prefer not requiring sorting for better performance.
// It allows passing hints that can help in optimising select, but it's up to implementation how this is used if used at all.
func (p *Querier) Select(bool, *storage.SelectHints, ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	return &seriesSet{}, nil, nil
}

type seriesSet struct {
}

func (s seriesSet) Next() bool         { return false }
func (s seriesSet) Err() error         { return nil }
func (s seriesSet) At() storage.Series { return nil }

// Appender implements storage.Appender.
type Appender struct {
}

// newAppender 实例化Appender
func newAppender() *Appender {
	a := Appender{}

	return &a
}

// Add adds a sample pair for the given series. A reference number is
// returned which can be used to add further samples in the same or later
// transactions.
// Returned reference numbers are ephemeral and may be rejected in calls
// to AddFast() at any point. Adding the sample via Add() returns a new
// reference number.
// If the reference is 0 it must not be used for caching.
func (a *Appender) Add(lsets labels.Labels, t int64, v float64) (uint64, error) {
	me := model.Metric{}
	for _, v := range lsets {
		me[model.LabelName(v.Name)] = model.LabelValue(v.Value)
	}

	s := model.Sample{
		Metric:    me,
		Value:     model.SampleValue(v),
		Timestamp: model.Time(t),
	}

	gOtsdbBuff <- &s

	return 0, nil
}

// AddFast adds a sample pair for the referenced series. It is generally
// faster than adding a sample by providing its full label set.
func (a *Appender) AddFast(ref uint64, t int64, v float64) error {
	return nil
}

// Commit submits the collected samples and purges the batch. If Commit
// returns a non-nil error, it also rolls back all modifications made in
// the appender so far, as Rollback would do. In any case, an Appender
// must not be used anymore after Commit has been called.
func (a *Appender) Commit() error {
	return nil
}

// Rollback rolls back all modifications made in the appender so far.
// Appender has to be discarded after rollback.
func (a *Appender) Rollback() error {
	return nil
}
