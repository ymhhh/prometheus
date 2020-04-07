// GNU GPL v3 License
// Copyright (c) 2016 github.com:go-trellis

package logger

const (
	// 默认的通道大小
	defaultChanBuffer int = 10000
)

// Debug 调试
func Debug(l Logger, msg string, fields ...interface{}) {
	l.Debug(msg, fields...)
}

// Debugf 调试
func Debugf(l Logger, msg string, fields ...interface{}) {
	l.Debugf(msg, fields...)
}

// Info 信息
func Info(l Logger, msg string, fields ...interface{}) {
	l.Info(msg, fields...)
}

// Infof 信息
func Infof(l Logger, msg string, fields ...interface{}) {
	l.Infof(msg, fields...)
}

// Error 错误
func Error(l Logger, msg string, fields ...interface{}) {
	l.Error(msg, fields...)
}

// Errorf 错误
func Errorf(l Logger, msg string, fields ...interface{}) {
	l.Errorf(msg, fields...)
}

// Warn 警告
func Warn(l Logger, msg string, fields ...interface{}) {
	l.Warn(msg, fields...)
}

// Warnf 警告
func Warnf(l Logger, msg string, fields ...interface{}) {
	l.Warnf(msg, fields...)
}

// Critical 异常
func Critical(l Logger, msg string, fields ...interface{}) {
	l.Critical(msg, fields...)
}

// Criticalf 异常
func Criticalf(l Logger, msg string, fields ...interface{}) {
	l.Criticalf(msg, fields...)
}

// With 增加默认的消息
func With(l Logger, params ...interface{}) Logger {
	return l.With(params...)
}

// WithPrefix 在最前面增加消息
func WithPrefix(l Logger, prefixes ...interface{}) Logger {
	return l.WithPrefix(prefixes...)
}
