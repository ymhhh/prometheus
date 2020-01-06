// GNU GPL v3 License
// Copyright (c) 2016 github.com:go-trellis

package formats

import (
	"net/http"
	"time"
)

// Datas
const (
	Date  = "2006-01-02"
	ZDate = "2006-1-2"

	DashTime = Date + "-15-04-05"
	DateTime = Date + " 15:04:05"
	Time     = "15:04:05"

	ChineseDate  = "2006年01月02日"
	ChineseZDate = "2006年1月2日"

	DefalutDateTime = "0001-01-01 00:00:00"
)

// MonthDays
const (
	MonthLunarDays   int = 30
	MonthSolarDays   int = 31
	MonthFebLeapDays int = 29
	MonthFebDays     int = 28
)

// FormatDate format date string
func FormatDate(t time.Time) string {
	return t.Format(Date)
}

// FormatChineseDate format chinese date string
func FormatChineseDate(t time.Time) string {

	return t.Format(ChineseDate)
}

// FormatZDate format no 0 date string
func FormatZDate(t time.Time) string {
	return t.Format(ZDate)
}

// FormatChineseZDate format chinese no 0 date string
func FormatChineseZDate(t time.Time) string {
	return t.Format(ChineseZDate)
}

// FormatTime format time string
func FormatTime(t time.Time) string {
	return t.Format(Time)
}

// FormatDateTime format datetime string
func FormatDateTime(t time.Time) string {
	return t.Format(DateTime)
}

// FormatDashTime format datetime string with dash
func FormatDashTime(t time.Time) string {
	return t.Format(DashTime)
}

// FormatRFC3339 format RFC3339 string
func FormatRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

// FormatRFC3339Nano format RFC3339Nano string
func FormatRFC3339Nano(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

// FormatGMT format GMT string
func FormatGMT(t time.Time) string {
	return t.Format(http.TimeFormat)
}

// IsZero judge time is zero
func IsZero(t time.Time) bool {
	return t.IsZero() || FormatTime(t) == DefalutDateTime
}

// GetTimeMonthDays get time's month days
func GetTimeMonthDays(t time.Time) int {
	return GetMonthDays(t.Year(), int(t.Month()))
}

// GetMonthDays get year's month days
func GetMonthDays(year, month int) int {
	switch month {
	case 2:
		if ((year%4) == 0 && (year%100) != 0) || (year%400) == 0 {
			return MonthFebLeapDays
		}
		return MonthFebDays
	case 4, 6, 9, 11:
		return MonthLunarDays
	}
	return MonthSolarDays
}

// StringToDate paser string to date
func StringToDate(t string) (time.Time, error) {
	return time.Parse(Date, t)
}

// StringToDateTime paser string to datetime
func StringToDateTime(t string) (time.Time, error) {
	return time.Parse(DateTime, t)
}

// UnixToTime Parse unix to time
func UnixToTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}
