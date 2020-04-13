// GNU GPL v3 License
// Copyright (c) 2018 github.com:go-trellis

package event

import "fmt"

// Subscriber 消费者
type Subscriber interface {
	GetID() string
	Publish(values ...interface{})
}

// NewDefSubscriber 生成默认的消费者
func NewDefSubscriber(sub interface{}) (Subscriber, error) {

	var subscriber Subscriber
	switch s := sub.(type) {
	case func(...interface{}):
		subscriber = &defSubscriber{
			id: GenSubscriberID(),
			fn: s,
		}
	case Subscriber:
		subscriber = s
	default:
		return nil, fmt.Errorf("unkown subscriber type: %+v", s)
	}
	return subscriber, nil
}

// Subscriber is returned from the Subscribe function.
//
// This value and can be passed to Unsubscribe when the observer is no longer interested in receiving messages
type defSubscriber struct {
	id string
	fn func(values ...interface{})
}

// GetID return Subscriber's id
func (p *defSubscriber) GetID() string {
	return p.id
}

// Publish 发布信息
func (p *defSubscriber) Publish(values ...interface{}) {
	p.fn(values...)
}
