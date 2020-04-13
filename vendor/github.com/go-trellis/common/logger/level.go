// GNU GPL v3 License
// Copyright (c) 2016 github.com:go-trellis

package logger

// Level log level
type Level int32

// define levels
const (
	MinLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	CriticalLevel

	LevelNameUnknown  = "Unknown"
	LevelNameDebug    = "DEBU"
	LevelNameInfo     = "INFO"
	LevelNameWarn     = "WARN"
	LevelNameError    = "ERRO"
	LevelNameCritical = "CRIT"

	levelColorDebug    = "\033[32m%s\033[0m" // grenn
	levelColorInfo     = "\033[37m%s\033[0m" // white
	levelColorWarn     = "\033[33m%s\033[0m" // yellow
	levelColorError    = "\033[31m%s\033[0m" // red
	levelColorCritical = "\033[35m%s\033[0m" // perple
)

// LevelColors printer's color
var LevelColors = map[Level]string{
	DebugLevel:    levelColorDebug,
	InfoLevel:     levelColorInfo,
	WarnLevel:     levelColorWarn,
	ErrorLevel:    levelColorError,
	CriticalLevel: levelColorCritical,
}

// ToLevelName 等级转换为名称
func ToLevelName(lvl Level) string {
	switch lvl {
	case DebugLevel:
		return LevelNameDebug
	case InfoLevel:
		return LevelNameInfo
	case WarnLevel:
		return LevelNameWarn
	case ErrorLevel:
		return LevelNameError
	case CriticalLevel:
		return LevelNameCritical
	default:
		return LevelNameUnknown
	}
}
