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

package opentsdb

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	connp "github.com/toolkits/conn_pool"
)

// OpenTsdb client
type OpenTsdbClient struct {
	cli    *struct{ net.Conn }
	name   string
	errCnt int
	errMtx sync.Mutex
	logger log.Logger
}

// Name 返回tsdb名称
func (t *OpenTsdbClient) Name() string {
	return t.name
}

// Closed 判断连接是否关闭
func (t *OpenTsdbClient) Closed() bool {
	return t.cli.Conn == nil
}

// Close 关闭当前连接
func (t *OpenTsdbClient) Close() error {
	if t.cli != nil {
		err := t.cli.Close()
		t.cli.Conn = nil
		level.Info(t.logger).Log("msg", "OpenTsdbClient close connect")
		return err
	}
	return nil
}

// SetErrCnt 设置错误数
func (t *OpenTsdbClient) SetErrCnt() {
	t.errMtx.Lock()
	defer t.errMtx.Unlock()
	t.errCnt++
}

// GetErrCnt 返回错误数
func (t *OpenTsdbClient) GetErrCnt() int {
	t.errMtx.Lock()
	defer t.errMtx.Unlock()
	errCnt := t.errCnt
	t.errCnt = 0
	return errCnt
}

// Read 读取opentsdb返回的数据
func (t *OpenTsdbClient) Read() {
	level.Info(t.logger).Log("msg", "OpenTsdbClient start read...")
	reader := bufio.NewReader(t.cli)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			level.Error(t.logger).Log("msg", "readString err", "err", err.Error())
			break
		} else {
			msg = strings.Trim(msg, "\n")
			level.Error(t.logger).Log("msg", "read err msg", "errMsg", msg)
			if strings.HasPrefix(msg, "put:") {
				t.SetErrCnt()
			}
		}
	}
	level.Info(t.logger).Log("msg", "OpenTsdbClient end read...")
}

// newConnPool 返回实例化 connp.ConnPool 对象
// address:     opentsdb地址，如 127.0.0.1:4242
// maxConns:    最大连接数
// maxIds:      最大空闲连接康悦
// connTimeout: 连接超时时间
func newConnPool(logger log.Logger, address string, maxConns, maxIdle int, connTimeout time.Duration) *connp.ConnPool {
	pool := connp.NewConnPool("opentsdb", address, int32(maxConns), int32(maxIdle))
	pool.New = func(name string) (connp.NConn, error) {
		// 验证address
		_, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			level.Error(logger).Log("msg", "ResolveTCPAddr err", "err", err.Error(), "address", address)
			return nil, err
		}

		// 连接opentsdb
		conn, err := net.DialTimeout("tcp", address, connTimeout)
		if err != nil {
			level.Error(logger).Log("msg", "DialTimeout err", "err", err.Error(), "address", address)
			return nil, err
		}

		tsdbClient := OpenTsdbClient{
			cli:    &struct{ net.Conn }{conn},
			name:   name,
			errCnt: 0,
			logger: logger,
		}
		go tsdbClient.Read()

		return &tsdbClient, nil
	}

	return pool
}

// TelnetPool
type TelnetPool struct {
	p           *connp.ConnPool
	maxConns    int
	maxIdle     int
	connTimeout time.Duration
	callTimeout time.Duration
	address     string
}

// NewTelnetPool 实例化 TelnetPool 对象
// address:     opentsdb地址，如 127.0.0.1:4242
// maxConns:    最大连接数
// maxIds:      最大空闲连接康悦
// connTimeout: 连接超时时间
// callTimeout: 发送超时时间
func NewTelnetPool(logger log.Logger, address string, maxConns, maxIdle int, connTimeout, callTimeout time.Duration) *TelnetPool {
	return &TelnetPool{
		p:           newConnPool(logger, address, maxConns, maxIdle, connTimeout),
		maxConns:    maxConns,
		maxIdle:     maxIdle,
		connTimeout: connTimeout,
		callTimeout: callTimeout,
		address:     address,
	}

}

// Send 发送数据，格式如下：
// put Metric Timestamp Value Tags(lable1=xxx lable2=xxx)
func (t *TelnetPool) Send(data []byte) (errCnt int, err error) {
	errCnt = 0
	conn, err := t.p.Fetch()
	if err != nil {
		return errCnt, fmt.Errorf("get connection fail: err %v. proc: %s", err, t.p.Proc())
	}

	cli := conn.(*OpenTsdbClient).cli
	done := make(chan error, 1)
	go func() {
		_, err = cli.Write(data)
		done <- err
	}()

	select {
	case <-time.After(t.callTimeout):
		// 发送超时强制关闭连接
		t.p.ForceClose(conn)
		return errCnt, fmt.Errorf("%s, call timeout", t.address)
	case err = <-done:
		if err != nil {
			// 发送出错时强制关闭连接
			t.p.ForceClose(conn)
			err = fmt.Errorf("%s, call failed, err %v. proc: %s", t.address, err, t.p.Proc())
		} else {
			errCnt = conn.(*OpenTsdbClient).GetErrCnt()
			t.p.Release(conn)
		}
		return errCnt, err
	}
}

// Destroy 销毁所在连接
func (t *TelnetPool) Destroy() {
	if t.p != nil {
		t.p.Destroy()
	}
}
