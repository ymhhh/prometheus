package models

type BzMonitorLabels struct {
	Id        int64  `xorm:"pk autoincr BIGINT(20)"`
	MonitorId string `xorm:"not null default '' index VARCHAR(50)"`
	Label     string `xorm:"not null default '' VARCHAR(100)"`
	Operator  string `xorm:"not null default '=' VARCHAR(20)"`
	Value     string `xorm:"not null default '' VARCHAR(100)"`
}
