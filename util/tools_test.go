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
	"testing"
)

// TestCheckMetircName 测试CheckMetircName函数
func TestCheckMetircName(t *testing.T) {
	name := "bdp_001"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = "3K_bdp_001"
	if CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: true expected: false", name)
		return
	}

	name = "bdp-001"
	if CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: true expected: false", name)
		return
	}

	name = "b"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = "b_"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = ":b_"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = ":b_:::123_sdf"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}
}
