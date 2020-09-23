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
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/shurcooL/httpfs/filter"
	"github.com/shurcooL/httpfs/union"
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

// Gzip zip压缩
// level:
// flate.NoCompression      = 0
// flate.BestSpeed          = 1
// flate.BestCompression    = 9
// flate.DefaultCompression = -1
func Gzip(src []byte, level int) ([]byte, error) {
	zw := bytes.NewBuffer(nil)
	gzipWriter, err := gzip.NewWriterLevel(zw, level)
	if err != nil {
		return nil, err
	}

	_, err = gzipWriter.Write(src)
	if err != nil {
		gzipWriter.Close()
		return nil, err
	}
	gzipWriter.Close()

	return zw.Bytes(), nil
}

// UnGzip zip解压
func UnGzip(src []byte) ([]byte, error) {
	r := bytes.NewReader(src)
	body, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getIPFromAddr(addr net.Addr) net.IP {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	}
	if ip == nil || ip.IsLoopback() {
		return nil
	}
	ip = ip.To4()
	if ip == nil {
		return nil // not an ipv4 address
	}

	return ip
}

// ExternalIP 获取本机IP
func ExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			ip := getIPFromAddr(addr)
			if ip == nil {
				continue
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("connected to the network?")
}

// IsIpv4 检验字符串是否是ipv4
//   参数
//     ip: IP地址
//   返回
//     是否IPv6地址
func IsIpv4(ip string) bool {
	pattern := `^((2[0-4]\d|25[0-5]|[01]?\d\d?)\.){3}(2[0-4]\d|25[0-5]|[01]?\d\d?)$`
	if m, _ := regexp.MatchString(pattern, ip); m {
		return true
	}

	return false
}

// IpItoa IP整型转字符串
// 如: "10.58.1.29" => 171573533
//   参数
//     iIp: IP整型形式
//   返回
//     IP字符串形式
func IpItoa(iIp int64) string {
	var bs [4]byte
	bs[0] = byte(iIp & 0xFF)
	bs[1] = byte((iIp >> 8) & 0xFF)
	bs[2] = byte((iIp >> 16) & 0xFF)
	bs[3] = byte((iIp >> 24) & 0xFF)

	return net.IPv4(bs[3], bs[2], bs[1], bs[0]).String()
}

// IpAtoi IP字符串转整型
// 如: 171573533 => "10.58.1.29"
//   参数
//     sIp: IP字符串形式
//   返回
//     IP整型形式
func IpAtoi(sIp string) int64 {
	if !IsIpv4(sIp) {
		return -1
	}

	bits := strings.Split(sIp, ".")
	b0, _ := strconv.Atoi(bits[0])
	b1, _ := strconv.Atoi(bits[1])
	b2, _ := strconv.Atoi(bits[2])
	b3, _ := strconv.Atoi(bits[3])

	var sum int64

	sum += int64(b0) << 24
	sum += int64(b1) << 16
	sum += int64(b2) << 8
	sum += int64(b3)

	return sum
}

//Md5 生成32位md5串
//   参数
//     s: 要加密的串
//   返回
//     md5后的结果
func Md5(s string) string {
	h := md5.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
