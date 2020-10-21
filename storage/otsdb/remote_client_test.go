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
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

// TestSamplesToProto 测试samplesToProto函数
func TestSamplesToProto(t *testing.T) {
	c := NewRemoteClient(nil, "http://localhost", 5*time.Second, map[string]string{})
	n1 := (model.Time(time.Now().Unix()) - 10) * 1000
	t11 := model.Sample{
		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "olap"},
		Value:     11.00,
		Timestamp: n1,
	}
	t12 := model.Sample{
		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "click"},
		Value:     12.00,
		Timestamp: n1,
	}
	n2 := model.Time(time.Now().Unix()) * 1000
	t21 := model.Sample{
		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "olap"},
		Value:     21.00,
		Timestamp: n2,
	}
	t22 := model.Sample{
		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "click"},
		Value:     22.00,
		Timestamp: n2,
	}
	s1 := model.Samples{&t11, &t12, &t21, &t22}
	s2 := c.samplesToProto(s1)
	t.Log("s2:", s2)
}

// TestWrite 测试Write函数
//func TestWrite(t *testing.T) {
//	c := NewRemoteClient(nil, "http://localhost:9111/api/prom/push", 5*time.Second, map[string]string{})
//	n1 := (model.Time(time.Now().Unix())) * 1000
//	t11 := model.Sample{
//		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "olap"},
//		Value:     11.00,
//		Timestamp: n1,
//	}
//	t12 := model.Sample{
//		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "click"},
//		Value:     12.00,
//		Timestamp: n1,
//	}
//	n2 := model.Time(time.Now().Unix()) * 1000
//	t21 := model.Sample{
//		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "olap"},
//		Value:     21.00,
//		Timestamp: n2,
//	}
//	t22 := model.Sample{
//		Metric:    model.Metric{"__name__": "test111", "cluster": "hope", "prod": "click"},
//		Value:     22.00,
//		Timestamp: n2,
//	}
//	s1 := model.Samples{&t11, &t12, &t21, &t22}
//	_, err := c.Write(s1)
//	if err != nil {
//		t.Log("err:", err)
//	}
//}
