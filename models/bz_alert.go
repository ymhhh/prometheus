package models

import (
	"time"
)

type BzAlert struct {
	Id             string    `xorm:"not null pk default '' VARCHAR(50)"`
	MonitorId      string    `xorm:"not null default '' index VARCHAR(50)"`
	Name           string    `xorm:"not null default '' VARCHAR(100)"`
	CreatorErp     string    `xorm:"not null default '' VARCHAR(50)"`
	Expression     string    `xorm:"TEXT"`
	Title          string    `xorm:"VARCHAR(50)"`
	Content        string    `xorm:"TEXT"`
	Status         string    `xorm:"not null default '' VARCHAR(50)"`
	Operator       string    `xorm:"comment('操作符：=, >, <, !=, <=, >=, between(介于), not_between(不介于)') VARCHAR(10)"`
	IsUsing        int       `xorm:"not null default 0 TINYINT(1)"`
	CreatedAt      time.Time `xorm:"not null default CURRENT_TIMESTAMP DATETIME"`
	UpdatedAt      time.Time `xorm:"not null default CURRENT_TIMESTAMP DATETIME"`
	Version        int       `xorm:"INT(11)"`
	PxAlertGroupId string    `xorm:"comment('phenix告警组ID备份') VARCHAR(50)"`
}
