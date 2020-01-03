# Times

## Feature

* FormatDate | "2016-01-01"
* FormatChineseDate | "2016年01月01日"
* FormatZDate | "2016-1-1"
* FormatChineseZDate | "2016年1月1日"
* FormatTime | "10:10:10"
* FormatDateTime | "2016-01-01 10:10:10"
* FormatDashTime | "2016-01-01-10-10-10"
* FormatRFC3339 | "2016-01-01T10:10:10Z"
* FormatRFC3339Nano | "2016-01-01T10:10:10.000000001Z"
* TimeToStringGMT | "Fri, 01 Jan 2016 10:10:10 GMT"
* IsZero | bool
* GetTimeMonthDays | times.MonthLunarDays
* GetMonthDays | times.MonthLunarDays

## Const

```go
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

const (
	MonthLunarDays   int = 30
	MonthSolarDays   int = 31
	MonthFebLeapDays int = 29
	MonthFebDays     int = 28
)
```