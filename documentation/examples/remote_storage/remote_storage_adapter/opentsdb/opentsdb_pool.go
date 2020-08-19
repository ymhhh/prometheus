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
	"container/list"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

var (
	nowFunc = time.Now
)

// OpenTsdb client
type OpenTsdbClient struct {
	cli    *struct{ net.Conn }
	errCnt int
	errMtx sync.Mutex
	logger log.Logger
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
	if err := recover(); err != nil {
		level.Info(t.logger).Log("msg", "OpenTsdbClient read recover", "err", err)
		return
	}

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

// stIdleConn 空闲连接结构
type stIdleConn struct {
	c *OpenTsdbClient
	t time.Time
}

// OpentsPool opentsdb连接池
type OpentsPool struct {
	addr        string        // opentsdb地址
	maxConns    int           // 最大连接数
	maxIdle     int           // 最大空闲连接数
	idleConns   list.List     // 空闲连接列表
	idleTimeout time.Duration // 最大空闲超时时间
	connTimeout time.Duration // 连接超时时间

	mu       sync.RWMutex
	cond     *sync.Cond
	wait     bool // 为true时，没有连接则等待，否则返回错误
	curConns int  // 打开的连接数
	closed   bool // 连接池是否已关闭
	logger   log.Logger
}

// NewOpentsdbPool 实例化opentsdb连接池
//  参数
//    addr:        opentsdb地址
//    maxConns:    最大连接数
//    maxIdle:     最大空闲连接数
//    idleTimeout: 空间连接的时间，超过这个时间就删除这个连接
//    wait:        没有空间连接且连接已超过最大连接数，是否等待，如果不等待就直接返回错
func NewOpentsdbPool(logger log.Logger, addr string, maxConns, maxIdle int, idleTimeout, connTimeout time.Duration, wait bool) *OpentsPool {
	p := OpentsPool{
		addr:        addr,
		maxConns:    maxConns,
		maxIdle:     maxIdle,
		idleTimeout: idleTimeout,
		connTimeout: connTimeout,

		curConns: 0,
		closed:   false,
		wait:     wait,
	}

	if logger == nil {
		p.logger = log.NewNopLogger()
	} else {
		p.logger = logger
	}

	if p.wait {
		cmu := sync.Mutex{}
		p.cond = sync.NewCond(&cmu)
	}

	return &p
}

// newConn 新的连接
func (p *OpentsPool) newConn() (*OpenTsdbClient, error) {
	// 验证address
	_, err := net.ResolveTCPAddr("tcp", p.addr)
	if err != nil {
		return nil, err
	}

	// 连接opentsdb
	conn, err := net.DialTimeout("tcp", p.addr, p.connTimeout)
	if err != nil {
		return nil, err
	}

	cli := OpenTsdbClient{
		cli:    &struct{ net.Conn }{conn},
		errCnt: 0,
		errMtx: sync.Mutex{},
		logger: p.logger,
	}
	go cli.Read()

	return &cli, err
}

// Get 返回一个连接
//   参数
//     void
//   返回
//     成功时返回连接串，失败返回错误信息
func (p *OpentsPool) Get() (*OpenTsdbClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 查询超时连接
	if p.idleTimeout > 0 {
		for i, n := 0, p.idleConns.Len(); i < n; i++ {
			e := p.idleConns.Back()
			if e == nil {
				break
			}
			ic := e.Value.(stIdleConn)
			if ic.t.Add(p.idleTimeout).After(nowFunc()) {
				// 未超时
				break
			}
			level.Info(p.logger).Log("msg", "get a timeout connect...")
			p.idleConns.Remove(e)
			p.release()
			ic.c.Close()
		}
	}

	for {
		// 如果有空闲，则取一个空闲连接
		if p.idleConns.Len() > 0 {
			e := p.idleConns.Front()
			if e == nil {
				level.Error(p.logger).Log("msg", "get a connect is nil")
			} else {
				p.idleConns.Remove(e)
				ic, ok := e.Value.(stIdleConn)
				if ok {
					// todo: 可以做一个测试连接的可用性
					level.Debug(p.logger).Log("msg", "get a connect is ok...")
					return ic.c, nil
				}
				level.Error(p.logger).Log("msg", "get a connect fmt is error")
			}
		}

		// 判断连接池是否关闭
		if p.closed {
			return nil, errors.New("connect pool is closed")
		}

		// 没有空闲，但连接数小于最大连接数，则新建一个连接
		if p.maxConns == 0 || p.curConns < p.maxConns {
			c, err := p.newConn()
			if err != nil {
				level.Error(p.logger).Log("msg", "new conn error", "err", err.Error(), "address", p.addr)
				return nil, err
			} else if c == nil {
				level.Error(p.logger).Log("msg", "new conn error", "err", "conn is nil")
				return c, errors.New("conn is nil")
			}

			p.curConns += 1
			level.Debug(p.logger).Log("msg", "new conn ok...")

			return c, nil
		}
		level.Warn(p.logger).Log("msg", "conn nums is max")

		// 等待连接
		if p.wait {
			level.Info(p.logger).Log("msg", "get conn wait...")
			p.mu.Unlock()
			p.cond.L.Lock()
			p.cond.Wait()
			p.cond.L.Unlock()
			p.mu.Lock()
			continue
		}

		// 如果不等待直接返回错误
		level.Error(p.logger).Log("msg", "no connection available")
		return nil, errors.New("no connection available")
	}

	return nil, errors.New("unknow error")
}

// Put 将连接放回连接池
//   参数
//     c:          连接串
//     forceClose: 是否强制关闭
//   返回
//     void
func (p *OpentsPool) Put(c *OpenTsdbClient, forceClose bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果连接为nil，则连接数-1，并唤醒信号
	if c == nil {
		level.Error(p.logger).Log("msg", "conn is nil")
		p.release()
		return
	}

	isPut := false
	if !forceClose && !p.closed {
		p.idleConns.PushFront(stIdleConn{t: nowFunc(), c: c})
		if p.maxIdle > 0 && p.idleConns.Len() > p.maxIdle {
			// 空闲数超出最大空闲数，则直接删除
			c = p.idleConns.Remove(p.idleConns.Back()).(stIdleConn).c
			level.Warn(p.logger).Log("msg", "maximum connections exceeded")
		} else {
			c = nil
			isPut = true
		}
	}

	// 返回空闲池的直接唤醒
	if isPut {
		if p.wait {
			p.cond.Signal()
		}
		level.Debug(p.logger).Log("msg", "put a connect ok...")

		return
	} else if c != nil {
		level.Info(p.logger).Log("msg", "close a connect")
		c.Close()
	}

	p.release()

	return
}

// release 释放一个连接、过期时会调用
func (p *OpentsPool) release() {
	p.curConns -= 1
	level.Debug(p.logger).Log("msg", "release", "curConns", p.curConns, "idleConns", p.idleConns.Len())

	if p.wait {
		p.cond.Signal()
	}
}

func (p *OpentsPool) Proc() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return fmt.Sprintf("maxConns:%d,maxIdle:%d,curConns:%d,idleConns:%d", p.maxConns, p.maxIdle, p.curConns, p.idleConns.Len())
}

// Close 关闭连接池
func (p *OpentsPool) Close() {
	p.mu.Lock()
	idle := p.idleConns
	p.idleConns.Init()
	p.closed = true
	//kp.curConns -= idle.Len()
	p.curConns -= 0
	if p.wait {
		p.cond.Broadcast()
	}
	p.mu.Unlock()
	for e := idle.Front(); e != nil; e = e.Next() {
		e.Value.(stIdleConn).c.Close()
	}
	level.Info(p.logger).Log("msg", "close all connect...")
	return
}

// TelnetPool
type TelnetPool struct {
	openPool     *OpentsPool
	maxConns     int
	maxIdle      int
	connTimeout  time.Duration
	writeTimeout time.Duration
	addr         string
}

// NewTelnetPool 实例化 TelnetPool 对象
// addr:         opentsdb地址，如 127.0.0.1:4242
// maxConns:     最大连接数
// maxIds:       最大空闲连接数
// idleTimeout:  空间超时时间
// connTimeout:  连接超时时间
// writeTimeout: 发送超时时间
func NewTelnetPool(logger log.Logger, addr string, maxConns, maxIdle int, idleTimeout, connTimeout, writeTimeout time.Duration, wait bool) *TelnetPool {
	return &TelnetPool{
		openPool:     NewOpentsdbPool(logger, addr, maxConns, maxIdle, idleTimeout, connTimeout, wait),
		maxConns:     maxConns,
		maxIdle:      maxIdle,
		connTimeout:  connTimeout,
		writeTimeout: writeTimeout,
		addr:         addr,
	}
}

// Send 发送数据，格式如下：
// put Metric Timestamp Value Tags(lable1=xxx lable2=xxx)
func (p *TelnetPool) Send(data []byte) (errCnt int, err error) {
	errCnt = 0
	conn, err := p.openPool.Get()
	if err != nil {
		return errCnt, fmt.Errorf("get connection fail: err %v. proc: %s", err, p.openPool.Proc())
	}
	done := make(chan error, 1)
	go func() {
		_, err = conn.cli.Write(data)
		done <- err
	}()

	select {
	case <-time.After(p.writeTimeout):
		// 发送超时强制关闭连接
		p.openPool.Put(conn, true)
		return errCnt, fmt.Errorf("%s, call timeout", p.addr)
	case err = <-done:
		if err != nil {
			// 发送出错时强制关闭连接，在外面会获取错误条数
			p.openPool.Put(conn, true)
			err = fmt.Errorf("%s, call failed, err %v. proc: %s", p.addr, err, p.openPool.Proc())
		} else {
			errCnt = conn.GetErrCnt()
			p.openPool.Put(conn, false)
		}
		return errCnt, err
	}
}

// Destroy 销毁所在连接
func (p *TelnetPool) Destroy() {
	if p.openPool != nil {
		p.openPool.Close()
	}
}
