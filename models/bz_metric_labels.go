package models

import (
	"time"
)

type BzMetricLabels struct {
	Id        int32     `xorm:"not null pk autoincr INT(11)"`
	ServiceId string    `xorm:"not null default '' comment('服务ID') unique(uniq_keys) VARCHAR(50)"`
	Metric    string    `xorm:"not null default '' comment('指标名') index(metric_label_idx) unique(uniq_keys) VARCHAR(100)"`
	Label     string    `xorm:"not null default '' comment('标签') index(metric_label_idx) unique(uniq_keys) VARCHAR(100)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
}
