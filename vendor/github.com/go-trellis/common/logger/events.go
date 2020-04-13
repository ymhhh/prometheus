// GNU GPL v3 License
// Copyright (c) 2016 github.com:go-trellis

package logger

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/go-trellis/common/event"
)

// Subscriber 注册个人的操作函数
func Subscriber(g event.SubscriberGroup, fn func(...interface{})) (event.Subscriber, error) {
	return g.Subscriber(fn)
}

// RemoveSubscriber 删除个人的操作函数
func RemoveSubscriber(g event.SubscriberGroup, ids ...string) error {
	return g.RemoveSubscriber(ids...)
}

// ClearSubscribers 释放所有的对象
func ClearSubscribers(g event.SubscriberGroup) {
	g.ClearSubscribers()
}

// Event log message
type Event struct {
	Time     time.Time
	Level    Level
	Prefixes []interface{}
	Fields   []interface{}
}

// Stack stores a stacktrace under the key "stacktrace".
func Stack() interface{} {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(5, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	var str string
	switch {
	case name != "":
		str = fmt.Sprintf("%v:%v", name, line)
	case file != "":
		str = fmt.Sprintf("%v:%v", file, line)
	default:
		str = fmt.Sprintf("pc:%x", pc)
	}
	return str
}
