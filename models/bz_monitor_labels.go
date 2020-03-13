package models

type BzMonitorLabels struct {
	Id        string `xorm:"not null pk default '' VARCHAR(50)"`
	MonitorId string `xorm:"not null default '' VARCHAR(50)"`
	Labels    string `xorm:"TEXT"`
}
