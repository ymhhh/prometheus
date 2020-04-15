package models

import (
	"time"
)

type BzAlert struct {
	Id              string    `xorm:"not null pk default '' VARCHAR(50)"`
	MonitorId       string    `xorm:"not null default '' index VARCHAR(50)"`
	Name            string    `xorm:"not null default '' VARCHAR(100)"`
	CreatorErp      string    `xorm:"not null default '' VARCHAR(50)"`
	UpdaterErp      string    `xorm:"VARCHAR(50)"`
	Severity        string    `xorm:"VARCHAR(50)"`
	Title           string    `xorm:"VARCHAR(50)"`
	Content         string    `xorm:"TEXT"`
	Status          string    `xorm:"not null default '' VARCHAR(50)"`
	IsUsing         int32     `xorm:"not null default 0 TINYINT(1)"`
	CreatedAt       time.Time `xorm:"DATETIME created"`
	UpdatedAt       time.Time `xorm:"DATETIME updated"`
	PxAlertGroupId  string    `xorm:"comment('phenix告警组ID备份') VARCHAR(50)"`
	IsContinueAlarm int32     `xorm:"comment('是否持续告警，0否，1是') TINYINT(1)"`
	GroupInterval   string    `xorm:"not null default '1m' comment('归组合并的时间，默认1分钟') VARCHAR(50)"`
	RepeatInterval  string    `xorm:"not null default '1s' comment('默认的下次发送的时间，秒级，必须大于0') VARCHAR(50)"`
	GroupBy         string    `xorm:"TEXT"`
	For             string    `xorm:"not null default '10m' VARCHAR(50)"`
	Version         int       `xorm:"version"`
	Operator        string    `xorm:"comment('操作符：=, >, <, !=, <=, >=, between(介于), not_between(不介于)') VARCHAR(20)"`
	Expression      string    `xorm:"TEXT"`
}
