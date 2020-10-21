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
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/common/version"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/util"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
)

// Client allows sending batches of Prometheus samples to Remote.
type RemoteClient struct {
	logger        log.Logger
	remoteTimeout time.Duration
	writeUrl      string
	headers       map[string]string
}

// NewRemoteClient 返回 RemoteClient 实例
func NewRemoteClient(logger log.Logger, writeUrl string, remoteTimeout time.Duration, headers map[string]string) *RemoteClient {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	if remoteTimeout <= 0 {
		remoteTimeout = 10 * time.Second
	}

	if !strings.HasPrefix(writeUrl, "http") {
		writeUrl = "http://" + writeUrl
	}

	c := RemoteClient{
		logger:        logger,
		remoteTimeout: remoteTimeout,
		writeUrl:      writeUrl,
		headers:       headers,
	}

	return &c
}

// samplesToProto 将model.Samples格式转换成[]prompb.TimeSeries
func (c *RemoteClient) samplesToProto(samples model.Samples) []prompb.TimeSeries {
	if len(samples) == 0 {
		return nil
	}

	mTs := make(map[string]*prompb.TimeSeries)
	for _, s := range samples {
		ps := prompb.Sample{
			Value:     float64(s.Value),
			Timestamp: int64(s.Timestamp),
		}
		key := util.Md5(s.Metric.String())
		if ts, ok := mTs[key]; ok {
			// 存在
			ts.Samples = append(ts.Samples, ps)
		} else {
			// label 不存在
			labels := make([]prompb.Label, len(s.Metric))
			i := 0
			for name, value := range s.Metric {
				labels[i] = prompb.Label{
					Name:  string(name),
					Value: string(value),
				}
				i++
			}

			mTs[key] = &prompb.TimeSeries{
				Labels:  labels,
				Samples: []prompb.Sample{ps},
			}
		}
	}

	allTs := make([]prompb.TimeSeries, len(mTs))
	i := 0
	for _, ts := range mTs {
		allTs[i] = *ts
		i++
	}

	return allTs
}

// Write samples写remote接口
func (c *RemoteClient) Write(samples model.Samples) (int, error) {
	allCnt := len(samples)
	if allCnt == 0 {
		return 0, nil
	}

	// 转换格式
	ts := c.samplesToProto(samples)
	if len(ts) == 0 {
		return allCnt, errors.New("samplesToProto is empty")
	}

	// 生成json串
	wr := &prompb.WriteRequest{
		Timeseries: ts,
	}
	data, err := proto.Marshal(wr)
	if err != nil {
		level.Error(c.logger).Log("msg", "Marshal err", "err", err.Error(), "url", c.writeUrl)
		return allCnt, err
	}

	// 压缩
	buf := make([]byte, 0)
	compressed := snappy.Encode(buf, data)

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), c.remoteTimeout)
	defer cancel()

	// 发送数据
	req, err := http.NewRequest("POST", c.writeUrl, bytes.NewBuffer(compressed))
	if err != nil {
		level.Error(c.logger).Log("msg", "NewRequest err", "err", err.Error(), "url", c.writeUrl)
		return allCnt, err
	}
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", fmt.Sprintf("Prometheus/%s", version.Version))
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		level.Error(c.logger).Log("msg", "Do err", "err", err.Error(), "url", c.writeUrl)
		return allCnt, err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	// 验证结果
	if resp.StatusCode < 400 {
		return 0, nil
	}

	// 失败取body
	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return allCnt, err
	}

	return allCnt, errors.New(string(buf))
}

func (c *RemoteClient) Name() string {
	return "remote"
}

func (c *RemoteClient) Destroy() {
}
