package models

import (
	"time"
)

type BzMetricLabels struct {
	Id        int32     `xorm:"not null pk autoincr INT(11)"`
	ServiceId string    `xorm:"not null default '' comment('服务ID') unique(uniq_keys) VARCHAR(50)"`
	Metric    string    `xorm:"not null default '' comment('指标名') unique(uniq_keys) VARCHAR(200)"`
	Label     string    `xorm:"not null default '' comment('标签') unique(uniq_keys) VARCHAR(100)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
}
