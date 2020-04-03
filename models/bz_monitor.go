package models

import (
	"time"
)

type BzMonitor struct {
	Id          string    `xorm:"not null pk default '' VARCHAR(50)"`
	Name        string    `xorm:"not null default '' comment('Alert名称') VARCHAR(100)"`
	CreatorErp  string    `xorm:"not null default '' comment('创建人') index VARCHAR(50)"`
	Expression  string    `xorm:"comment('监控表达式') TEXT"`
	Description string    `xorm:"VARCHAR(100)"`
	Status      string    `xorm:"not null default '' comment('报警状态') index VARCHAR(50)"`
	IsUsing     int32     `xorm:"not null default 0 comment('是否启动：0否，1是') index TINYINT(1)"`
	CreatedAt   time.Time `xorm:"DATETIME created"`
	UpdatedAt   time.Time `xorm:"DATETIME updated"`
	Version     int       `xorm:"version"`
}
