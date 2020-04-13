// GNU GPL v3 License
// Copyright (c) 2018 github.com:go-trellis

package event

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// SubscriberModel 消费者模式
const (
	// 普通模式
	SubscriberModelNormal = iota
	// 并发模式
	SubscriberModelGoutine
)

// SubscriberGroup 消费者组
type SubscriberGroup interface {
	Subscriber(interface{}) (Subscriber, error)
	RemoveSubscriber(ids ...string) error
	Publish(values ...interface{})
	ClearSubscribers()
}

type defSubscriberGroup struct {
	locker      *sync.RWMutex
	subscribers map[string]Subscriber
	model       int
}

// GroupOption 操作配置函数
type GroupOption func(*defSubscriberGroup)

// GroupSubscriberModel 组的分享类型
func GroupSubscriberModel(model int) GroupOption {
	return func(g *defSubscriberGroup) {
		g.model = model
	}
}

// NewSubscriberGroup xxx
func NewSubscriberGroup(opts ...GroupOption) SubscriberGroup {
	g := &defSubscriberGroup{
		locker:      &sync.RWMutex{},
		subscribers: make(map[string]Subscriber),
	}
	for _, o := range opts {
		o(g)
	}

	return g
}

// Subscriber 注册消费者
func (p *defSubscriberGroup) Subscriber(sub interface{}) (Subscriber, error) {
	subscriber, err := NewDefSubscriber(sub)
	if err != nil {
		return nil, err
	}

	p.locker.Lock()
	defer p.locker.Unlock()

	p.subscribers[subscriber.GetID()] = subscriber
	return subscriber, nil
}

// GenSubscriberID 生成消费者ID
func GenSubscriberID() string {
	return uuid.New().URN()
}

// RemoveSubscriber xxx
func (p *defSubscriberGroup) RemoveSubscriber(ids ...string) error {
	if 0 == len(ids) {
		return errors.New("empty input sub ids")
	}
	p.locker.Lock()
	defer p.locker.Unlock()

	for _, v := range ids {
		if 0 == len(v) {
			return errors.New("empty sub id")
		}
		delete(p.subscribers, v)
	}

	return nil
}

// Publish 发布消息
func (p *defSubscriberGroup) Publish(values ...interface{}) {
	for _, sub := range p.subscribers {
		switch p.model {
		case SubscriberModelGoutine:
			go sub.Publish(values...)
		default:
			sub.Publish(values...)
		}
	}
}

// ClearSubscribers 全部清理
func (p *defSubscriberGroup) ClearSubscribers() {
	for key, sub := range p.subscribers {
		if sub == nil {
			delete(p.subscribers, key)
		}
	}
}
