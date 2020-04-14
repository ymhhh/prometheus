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
	"compress/flate"
	"testing"
)

// TestIsDigit 测试IsDigit函数
func TestIsDigit(t *testing.T) {
	src := ""
	if IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: true expected: false", src)
		return
	}

	src = "123test"
	if IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: true expected: false", src)
		return
	}

	src = "test"
	if IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: true expected: false", src)
		return
	}

	src = "123"
	if !IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: false expected: true", src)
		return
	}

	src = "0"
	if !IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: false expected: true", src)
		return
	}

	src = "9"
	if !IsDigit(src) {
		t.Errorf("IsDigit[%s] err, Got: false expected: true", src)
		return
	}
}

// TestIsHex 测试IsHex函数
func TestIsHex(t *testing.T) {
	src := ""
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "0"
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "0x"
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "0x123ZZ"
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "0b123"
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "123456"
	if IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "0x123bcd"
	if !IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "0X123BCD"
	if !IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "0x123"
	if !IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "0xBCD"
	if !IsHex(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}
}

// TestIsDate 测试IsDate函数
func TestIsDate(t *testing.T) {
	src := ""
	if IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "1"
	if IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "1b"
	if IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "1h123"
	if IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "1h123m"
	if IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: true expected: false", src)
		return
	}

	src = "1h"
	if !IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "10m"
	if !IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "0s"
	if !IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "2019y"
	if !IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}

	src = "99w"
	if !IsDate(src) {
		t.Errorf("IsHex[%s] err, Got: false expected: true", src)
		return
	}
}

// TestCheckMetircName 测试CheckMetircName函数
func TestCheckMetircName(t *testing.T) {
	name := "bdp_001"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = "3K_bdp_001"
	if !CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: false expected: true", name)
		return
	}

	name = "bdp-001"
	if CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: true expected: false", name)
		return
	}

	name = "9999"
	if CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: true expected: false", name)
		return
	}

	name = "0x999"
	if CheckMetircName(name) {
		t.Errorf("CheckMetircName[%s] err, Got: true expected: false", name)
		return
	}

	name = "9999y"
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

// TestGzip 测试Gzip函数
func TestGzip(t *testing.T) {
	src := "hello world!"
	b1, err := Gzip([]byte(src), flate.DefaultCompression)
	if err != nil {
		t.Errorf("Gzip err[%s]", err.Error())
		return
	}

	b2, err := UnGzip(b1)
	if err != nil {
		t.Errorf("UnGzip err[%s]", err.Error())
		return
	}

	if string(b2) != src {
		t.Errorf("Gzip err, Got: [%s]] expected: [%s]]", string(b2), src)
		return
	}
}
