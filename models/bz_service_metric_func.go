package models

import (
	"time"
)

type BzServiceMetricFunc struct {
	Id              int32     `xorm:"not null pk autoincr INT(11)"`
	ServiceId       string    `xorm:"not null default '' unique(uniq_keys) VARCHAR(50)"`
	Metric          string    `xorm:"not null default '' unique(uniq_keys) VARCHAR(100)"`
	ComputeFunction string    `xorm:"not null default '' comment('函数模板') VARCHAR(50)"`
	RangeVectors    string    `xorm:"not null default '' VARCHAR(11)"`
	CreatedAt       time.Time `xorm:"DATETIME created"`
}
