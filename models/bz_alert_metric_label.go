package models

type BzAlertMetricLabel struct {
	Id       string `xorm:"not null pk default '' VARCHAR(50)"`
	AlertId  string `xorm:"not null default '' index VARCHAR(50)"`
	Label    string `xorm:"not null default '' VARCHAR(200)"`
	Operator string `xorm:"not null default '' VARCHAR(10)"`
	Value    string `xorm:"TEXT"`
}
