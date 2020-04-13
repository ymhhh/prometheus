// GNU GPL v3 License
// Copyright (c) 2018 github.com:go-trellis

package event

// Bus xxx
type Bus interface {
	RegistEvent(eventNames ...string) error

	Subscribe(eventName string, fn func(...interface{})) (Subscriber, error)
	Unsubscribe(eventName string, ids ...string) error
	UnsubscribeAll(eventName string)

	Publish(eventName string, evt ...interface{})

	ListEvents() (events []string)
}

// DefaultEventCenterName default event center name
const DefaultEventCenterName = "trellis::event::default-center"

var defBus = NewEventCenter(DefaultEventCenterName)

// RegistEvent 注册事件
func RegistEvent(eventNames ...string) error {
	return defBus.RegistEvent(eventNames...)
}

// Subscribe 监听
func Subscribe(eventName string, fn func(...interface{})) (Subscriber, error) {
	return defBus.Subscribe(eventName, fn)
}

// Unsubscribe 取消监听
func Unsubscribe(eventName string, ids ...string) error {
	return defBus.Unsubscribe(eventName, ids...)
}

// Publish 发布消息
func Publish(eventName string, event ...interface{}) {
	defBus.Publish(eventName, event...)
}

// ListEvents 全部事件
func ListEvents() (events []string) {
	return defBus.ListEvents()
}
