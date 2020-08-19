// Copyright 2014 The Prometheus Authors
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

package util

import (
	"sync"
	"testing"
	"time"
)

// TestCount 测试Count
func TestCount(t *testing.T) {
	maxNum := 10
	c := Count{}
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		if c.Num() >= maxNum {
			t.Log("max curNum:", c.Num())
			break
		}

		t.Log("curNum:", c.Num())
		c.Inc()
		wg.Add(1)
		go func() {
			defer c.Dec()
			defer wg.Done()

			// dosomething...
			time.Sleep(1 * time.Second)
		}()
	}

	wg.Wait()
}
