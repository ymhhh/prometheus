// Copyright 2018 The Prometheus Authors
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
	"github.com/shurcooL/httpfs/filter"
	"github.com/shurcooL/httpfs/union"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// SetAssets 设置ui.Assets使用，从ui里迁移过来
func SetAssets(uiPath string) http.FileSystem {
	assetsPrefix := uiPath
	if assetsPrefix == "" {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		switch path.Base(wd) {
		case "prometheus":
			// When running Prometheus (without built-in assets) from the repo root.
			assetsPrefix = "./web/ui"
		case "web":
			// When running web tests.
			assetsPrefix = "./ui"
		case "ui":
			// When generating statically compiled-in assets.
			assetsPrefix = "./"
		}
	}

	static := filter.Keep(
		http.Dir(path.Join(assetsPrefix, "static")),
		func(path string, fi os.FileInfo) bool {
			return fi.IsDir() ||
				(!strings.HasSuffix(path, "map.js") &&
					!strings.HasSuffix(path, "/bootstrap.js") &&
					!strings.HasSuffix(path, "/bootstrap-theme.css") &&
					!strings.HasSuffix(path, "/bootstrap.css"))
		},
	)

	templates := filter.Keep(
		http.Dir(path.Join(assetsPrefix, "templates")),
		func(path string, fi os.FileInfo) bool {
			return fi.IsDir() || strings.HasSuffix(path, ".html")
		},
	)

	return union.New(map[string]http.FileSystem{
		"/templates": templates,
		"/static":    static,
	})
}

// IsDigit 判断是全是数字
func IsDigit(src string) bool {
	if src == "" {
		return false
	}

	for _, v := range src {
		if v < '0' || v > '9' {
			return false
		}
	}

	return true
}

// IsHex 判断是否16进制，如0x12BC
func IsHex(src string) bool {
	if len(src) < 3 {
		return false
	}

	if src[0] != '0' || !strings.ContainsRune("xX", rune(src[1])) {
		return false
	}

	digits := "0123456789abcdefABCDEF"
	for _, v := range src[2:] {
		if !strings.ContainsRune(digits, rune(v)) {
			return false
		}
	}

	return true
}

// IsDate 日期格式，前面是数字，后面是smhdwy中的一个，这里不验证单位前面数字的大小
func IsDate(src string) bool {
	l := len(src)
	if l < 2 {
		return false
	}

	if !strings.ContainsRune("smhdwy", rune(src[l-1])) {
		return false
	}

	if !IsDigit(src[:l-1]) {
		return false
	}

	return true
}

// CheckMetircName check the metric name
func CheckMetircName(name string) bool {
	if name == "" {
		return false
	}

	// 全是数字
	if IsDigit(name) {
		return false
	}

	// 16进制格式
	if IsHex(name) {
		return false
	}

	// 日期格式
	if IsDate(name) {
		return false
	}

	// 正则匹配
	//r, _ := regexp.Compile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	r, _ := regexp.Compile(`^[a-zA-Z0-9_:][a-zA-Z0-9_:]*$`)
	return r.MatchString(name)
}

// IsValidMetricName returns true iff name matches the pattern of MetricNameRE.
// This function, however, does not use MetricNameRE for the check but a much
// faster hardcoded implementation.
// from: vendor/githup.com/prometheus/common/models/metric.go
func IsValidMetricName(n string) bool {
	if len(n) == 0 {
		return false
	}
	for _, b := range n {
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == ':' || (b >= '0' && b <= '9')) {
			return false
		}
	}
	return true
}

