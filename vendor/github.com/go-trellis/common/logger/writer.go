// GNU GPL v3 License
// Copyright (c) 2016 github.com:go-trellis

package logger

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-trellis/common/event"
)

// Writer 写对象
type Writer interface {
	// io.Writer
	event.Subscriber
	Stop()
}

func generateLogs(evt *Event, separator string) string {

	logs := make([]string, 0, 2+len(evt.Prefixes)+len(evt.Fields))
	logs = append(logs, evt.Time.Format("2006/01/02T15:04:05.000"), ToLevelName(evt.Level))

	for _, pref := range evt.Prefixes {
		logs = append(logs, replacerString(pref.(string), separator))
	}

	for _, v := range evt.Fields {
		switch reflect.TypeOf(v).Kind() {
		case reflect.Ptr, reflect.Struct, reflect.Map:
			bs, err := json.Marshal(v)
			if err != nil {
				panic(err)
			}
			logs = append(logs, string(bs))
		default:
			logs = append(logs, fmt.Sprintf("%+v", v))
		}
	}
	return strings.Join(logs, separator) + "\n"
}

func replacerString(origin, replacer string) string {
	str := ReplaceString(origin, " ")
	return ReplaceString(str, replacer)
}

// ReplaceString 替换字符串
func ReplaceString(origin, replacer string) string {
	return strings.Replace(origin, replacer, "", -1)
}
