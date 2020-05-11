// Copyright 2013 The Prometheus Authors
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

package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/prometheus/prompb"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
)

const (
	putEndpoint     = "/api/put"
	queryEndpoint   = "/api/query"
	contentTypeJSON = "application/json"
)

// errMsg opentsdb error message
type errMsg struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// errRet opentsdb error return
type errRet struct {
	Error errMsg `json:"error"`
}

// Client allows sending batches of Prometheus samples to OpenTSDB.
type Client struct {
	logger   log.Logger
	timeout  time.Duration
	writeUrl string
	readUrl  string
	isRe     bool
}

// NewClient creates a new Client.
func NewClient(logger log.Logger, openWUrl, openRUrl string, timeout time.Duration, isRe bool) *Client {
	c := Client{
		logger:  logger,
		timeout: timeout,
		isRe:    isRe,
	}

	if openWUrl == "" {
		openWUrl = openRUrl
	} else if openRUrl == "" {
		openRUrl = openWUrl
	}

	// write url
	u, err := url.Parse(openWUrl)
	if err != nil {
		level.Error(c.logger).Log("msg", "parse write url err", "err", err.Error(), "url", openWUrl)
	}
	u.Path = putEndpoint
	c.writeUrl = u.String()

	// read url
	u, err = url.Parse(openRUrl)
	if err != nil {
		level.Error(c.logger).Log("msg", "parse read url err", "err", err.Error(), "url", openRUrl)
	}
	u.Path = queryEndpoint
	c.readUrl = u.String()

	return &c
}

// StoreSamplesRequest is used for building a JSON request for storing samples
// via the OpenTSDB.
type StoreSamplesRequest struct {
	Metric    TagValue            `json:"metric"`
	Timestamp int64               `json:"timestamp"`
	Value     float64             `json:"value"`
	Tags      map[string]TagValue `json:"tags"`
}

// tagsFromMetric translates Prometheus metric into OpenTSDB tags.
func tagsFromMetric(m model.Metric) map[string]TagValue {
	tags := make(map[string]TagValue, len(m)-1)
	for l, v := range m {
		if l == model.MetricNameLabel {
			continue
		}
		tags[string(l)] = TagValue(v)
	}
	return tags
}

// Write sends a batch of samples to OpenTSDB via its HTTP API.
func (c *Client) Write(samples model.Samples) (int, error) {
	reqs := make([]StoreSamplesRequest, 0, len(samples))
	for _, s := range samples {
		v := float64(s.Value)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			level.Debug(c.logger).Log("msg", "cannot send value to OpenTSDB, skipping sample", "value", v, "sample", s)
			continue
		}
		metric := TagValue(s.Metric[model.MetricNameLabel])
		reqs = append(reqs, StoreSamplesRequest{
			Metric:    metric,
			Timestamp: s.Timestamp.Unix(),
			Value:     v,
			Tags:      tagsFromMetric(s.Metric),
		})
	}

	buf, err := json.Marshal(reqs)
	if err != nil {
		level.Error(c.logger).Log("msg", "Marshal err", "err", err.Error(), "url", c.writeUrl)
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequest("POST", c.writeUrl, bytes.NewBuffer(buf))
	if err != nil {
		level.Error(c.logger).Log("msg", "NewRequest err", "err", err.Error(), "url", c.writeUrl)
		return 0, err
	}
	req.Header.Set("Content-Type", contentTypeJSON)
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		level.Error(c.logger).Log("msg", "Do err", "err", err.Error(), "url", c.writeUrl)
		return 0, err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	// API returns status code 204 for successful writes.
	// http://opentsdb.net/docs/build/html/api_http/put.html
	if resp.StatusCode < 400 {
		return len(samples), nil
	}

	// API returns status code 400 on error, encoding error details in the
	// response content in JSON.
	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var r map[string]int
	if err := json.Unmarshal(buf, &r); err != nil {
		st := errRet{}
		err = json.Unmarshal(buf, &st)
		if err != nil {
			return 0, err
		} else {
			return 0, fmt.Errorf("code:%d msg:%s", st.Error.Code, st.Error.Message)
		}

		return 0, err
	}
	return r["success"], errors.Errorf("failed to write %d samples to OpenTSDB, %d succeeded", r["failed"], r["success"])
}

// Name identifies the client as an OpenTSDB client.
func (c Client) Name() string {
	return "opentsdb"
}

func (c *Client) Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	queryReqs := make([]*otdbQueryReq, 0, len(req.Queries))
	smatchers := make(map[*otdbQueryReq]seriesMatcher)
	for _, q := range req.Queries {
		res, matcher, err := c.buildQueryReq(q, c.isRe)
		if err != nil {
			return nil, err
		}
		queryReqs = append(queryReqs, res)
		smatchers[res] = matcher
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	errCh := make(chan error, 1)
	defer close(errCh)
	var l sync.Mutex
	labelsToSeries := map[string]*prompb.TimeSeries{}
	for i := range queryReqs {
		go func(queryReq *otdbQueryReq) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			rawBytes, err := json.Marshal(queryReq)
			if err != nil {
				level.Error(c.logger).Log("msg", "marshal error", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}

			req, err := http.NewRequest("POST", c.readUrl, bytes.NewBuffer(rawBytes))
			if err != nil {
				level.Error(c.logger).Log("msg", "new request error", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}
			req.Header.Set("Content-Type", contentTypeJSON)

			resp, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				level.Error(c.logger).Log("msg", "falied to send request to opentsdb", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}

			defer func() {
				io.Copy(ioutil.Discard, resp.Body)
				resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				level.Error(c.logger).Log("msg", "query opentsdb error", "err", string(rawBytes), "http_code", resp.StatusCode, "url", c.readUrl)
				errCh <- fmt.Errorf("got status code %v", resp.StatusCode)
				return
			}

			rawBytes, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				level.Error(c.logger).Log("msg", "read all error", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}

			var res otdbQueryResSet

			if err = json.Unmarshal(rawBytes, &res); err != nil {
				level.Error(c.logger).Log("msg", "unmarshal error", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}

			// 合并结果
			err = func() error {
				l.Lock()
				defer l.Unlock()
				return mergeResult(labelsToSeries, smatchers[queryReq], &res)
			}()
			if err != nil {
				level.Error(c.logger).Log("msg", "merge result error", "err", err.Error(), "url", c.readUrl)
				errCh <- err
				return
			}

			errCh <- nil
		}(queryReqs[i])
	}

loop:
	for {
		count := 0
		select {
		case err := <-errCh:
			if err != nil {
				return nil, err
			}
			count++
			if count == len(queryReqs) {
				break loop
			}
		default:
		}
	}

	resp := prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{Timeseries: make([]*prompb.TimeSeries, 0, len(labelsToSeries))},
		},
	}
	for _, ts := range labelsToSeries {
		resp.Results[0].Timeseries = append(resp.Results[0].Timeseries, ts)
	}
	return &resp, nil
}

func (c *Client) buildQueryReq(q *prompb.Query, isRe bool) (*otdbQueryReq, seriesMatcher, error) {
	req := otdbQueryReq{
		Start: q.GetStartTimestampMs() / 1000,
		End:   q.GetEndTimestampMs() / 1000,
	}

	aggr := "none"
	qr := otdbQuery{
		Aggregator: aggr,
	}
	var smatcher seriesMatcher
	for _, m := range q.Matchers {
		// metric
		if m.Name == model.MetricNameLabel {
			switch m.Type {
			case prompb.LabelMatcher_EQ:
				qr.Metric = TagValue(m.Value)
			default:
				// TODO: Figure out how to support these efficiently.
				return nil, nil, fmt.Errorf("regex, non-equal or regex-non-equal matchers are not supported on the metric name yet")
			}
			continue
		}

		// label
		ft := otdbFilter{
			GroupBy: true,
			Tagk:    m.Name,
		}
		switch m.Type {
		case prompb.LabelMatcher_EQ:
			ft.Type = otdbFilterTypeLiteralOr
			ft.Filter = toTagValue(m.Value, false)
		case prompb.LabelMatcher_NEQ:
			ft.Type = otdbFilterTypeNotLiteralOr
			ft.Filter = toTagValue(m.Value, false)
		case prompb.LabelMatcher_RE:
			if isRe {
				// 目前支持带*或|的正则，如aa.*、aa.*|bb.*
				ft.Type = otdbFilterTypeRegexp
				ft.Filter = "^(?:" + toTagValue(m.Value, true) + ")$"
			} else {
				ft.Type = otdbFilterTypeWildcard
				ft.Filter = "*"
				tmp, err := NewLabelMatcher(RegexMatch, m.Name, m.Value)
				if err != nil {
					return nil, nil, fmt.Errorf("create matcher error: %v", err)
				}
				smatcher = append(smatcher, tmp)
			}
		case prompb.LabelMatcher_NRE:
			if isRe {
				// 目前支持带*或|的正则，如aa.*、aa.*|bb.*
				ft.Type = otdbFilterTypeRegexp
				ft.Filter = "^((?!" + toTagValue(m.Value, true) + ").)*$"
			} else {
				ft.Type = otdbFilterTypeWildcard
				ft.Filter = "*"
				tmp, err := NewLabelMatcher(RegexNoMatch, m.Name, m.Value)
				if err != nil {
					return nil, nil, fmt.Errorf("create matcher error: %v", err)
				}
				smatcher = append(smatcher, tmp)
			}
		default:
			return nil, nil, fmt.Errorf("unknown match type %v", m.Type)
		}
		qr.Filters = append(qr.Filters, ft)
	}

	req.Queries = append(req.Queries, qr)
	return &req, smatcher, nil
}

func concatLabels(labels map[string]TagValue) string {
	// 0xff cannot cannot occur in valid UTF-8 sequences, so use it
	// as a separator here.
	separator := "\xff"
	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, k+separator+string(v))
	}
	return strings.Join(pairs, separator)
}

func tagsToLabelPairs(name string, tags map[string]TagValue) []prompb.Label {
	pairs := make([]prompb.Label, 0, len(tags))
	for k, v := range tags {
		pairs = append(pairs, prompb.Label{
			Name:  k,
			Value: string(v),
		})
	}
	pairs = append(pairs, prompb.Label{
		Name:  model.MetricNameLabel,
		Value: name,
	})
	return pairs
}

// mergeSamples merges two lists of sample pairs and removes duplicate
// timestamps. It assumes that both lists are sorted by timestamp.
func mergeSamples(a, b []prompb.Sample) []prompb.Sample {
	result := make([]prompb.Sample, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].Timestamp < b[j].Timestamp {
			result = append(result, a[i])
			i++
		} else if a[i].Timestamp > b[j].Timestamp {
			result = append(result, b[j])
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func valuesToSamples(dps otdbDPs) []prompb.Sample {
	samples := make([]prompb.Sample, 0, len(dps))
	for t, v := range dps {
		samples = append(samples, prompb.Sample{
			Timestamp: t * 1000,
			Value:     v,
		})
	}
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].Timestamp != samples[j].Timestamp {
			return samples[i].Timestamp < samples[j].Timestamp
		}
		return samples[i].Value < samples[j].Value
	})
	return samples
}

func mergeResult(labelsToSeries map[string]*prompb.TimeSeries, smatcher seriesMatcher, results *otdbQueryResSet) error {
	var series otdbQueryResSet
	if smatcher == nil {
		series = *results
	} else {
		series = make([]*otdbQueryRes, 0, len(*results))
		for _, r := range *results {
			if smatcher.Match(r.Tags) {
				series = append(series, r)
			}
		}
	}
	for _, r := range series {
		k := concatLabels(r.Tags)
		ts, ok := labelsToSeries[k]
		if !ok {
			ts = &prompb.TimeSeries{
				Labels: tagsToLabelPairs(string(r.Metric), r.Tags),
			}
			labelsToSeries[k] = ts
		}
		ts.Samples = mergeSamples(ts.Samples, valuesToSamples(r.DPs))
	}
	return nil
}
