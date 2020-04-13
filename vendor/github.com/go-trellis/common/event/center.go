// GNU GPL v3 License
// Copyright (c) 2018 github.com:go-trellis

package event

import (
	"errors"
	"fmt"
	"sync"
)

// Center xxx
type Center struct {
	locker *sync.RWMutex
	name   string
	groups map[string]SubscriberGroup
}

// NewEventCenter xxx
func NewEventCenter(name string) Bus {
	if 0 == len(name) {
		panic(errors.New("center name is empty"))
	}
	return &Center{
		locker: &sync.RWMutex{},
		groups: make(map[string]SubscriberGroup),
	}
}

// Name center name
func (p *Center) Name() string {
	return p.name
}

// RegistEvent 注册事件
func (p *Center) RegistEvent(eventNames ...string) error {
	if len(eventNames) == 0 {
		return nil
	}

	p.locker.Lock()
	defer p.locker.Unlock()
	for _, eventName := range eventNames {
		if len(eventName) == 0 {
			return errors.New("center event is empty")
		}

		if _, exist := p.groups[eventName]; exist {
			return fmt.Errorf("event name [%s] is already in groups", eventName)
		}

		p.groups[eventName] = NewSubscriberGroup()
	}
	return nil
}

// Subscribe 监听
func (p *Center) Subscribe(eventName string, fn func(...interface{})) (Subscriber, error) {
	if len(eventName) == 0 {
		return nil, errors.New("event name is empty")
	}
	p.locker.RLock()
	defer p.locker.RUnlock()
	group, exist := p.groups[eventName]
	if !exist {
		return nil, fmt.Errorf("event name [%s] is not exists", eventName)
	}
	return group.Subscriber(fn)
}

// Unsubscribe 取消监听
func (p *Center) Unsubscribe(eventName string, ids ...string) error {
	if len(eventName) == 0 {
		return errors.New("event name is empty")
	}
	p.locker.Lock()
	defer p.locker.Unlock()
	group, exist := p.groups[eventName]
	if !exist {
		return fmt.Errorf("event name [%s] is not exists", eventName)
	}

	return group.RemoveSubscriber(ids...)
}

// UnsubscribeAll 取消全部监听
func (p *Center) UnsubscribeAll(eventName string) {
	p.locker.Lock()
	defer p.locker.Unlock()
	group, exist := p.groups[eventName]
	if !exist {
		return
	}
	group.ClearSubscribers()
}

// Publish 分发
func (p *Center) Publish(eventName string, evts ...interface{}) {
	if len(eventName) == 0 {
		return
	}

	p.locker.RLock()
	defer p.locker.RUnlock()
	group, exist := p.groups[eventName]
	if !exist {
		return
	}

	group.Publish(evts...)
}

// ListEvents 全部事件
func (p *Center) ListEvents() (events []string) {
	p.locker.RLock()
	defer p.locker.RUnlock()
	for event := range p.groups {
		events = append(events, event)
	}
	return
}
