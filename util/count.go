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

import "sync"

type Count struct {
	lockNum sync.Mutex // lock
	curNums int        // current number
}

// Num return current number
func (c *Count) Num() int {
	c.lockNum.Lock()
	defer c.lockNum.Unlock()
	return c.curNums
}

// Inc increase current number
func (c *Count) Inc() int {
	c.lockNum.Lock()
	defer c.lockNum.Unlock()
	c.curNums++
	return c.curNums
}

// Dec decrement current number
func (c *Count) Dec() int {
	c.lockNum.Lock()
	defer c.lockNum.Unlock()
	c.curNums--
	return c.curNums
}
