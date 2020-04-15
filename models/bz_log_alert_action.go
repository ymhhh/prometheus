package models

import (
	"time"
)

type BzLogAlertAction struct {
	Id          int64     `xorm:"pk autoincr BIGINT(20)"`
	AlertId     string    `xorm:"not null default '' VARCHAR(50)"`
	AlertName   string    `xorm:"VARCHAR(100)"`
	MonitorId   string    `xorm:"VARCHAR(50)"`
	MonitorName string    `xorm:"VARCHAR(100)"`
	ActionId    int32     `xorm:"not null default 0 INT(11)"`
	Severity    string    `xorm:"not null default 'normal' comment('报警等级：0普通，1告警，2紧急') VARCHAR(50)"`
	Expression  string    `xorm:"not null comment('监控表达式') TEXT"`
	CallbackUrl string    `xorm:"TEXT"`
	Status      string    `xorm:"not null default '' VARCHAR(50)"`
	Erps        string    `xorm:"VARCHAR(1000)"`
	Title       string    `xorm:"comment('发送标题') VARCHAR(150)"`
	Content     string    `xorm:"comment('发送的内容') VARCHAR(1500)"`
	CreatedAt   time.Time `xorm:"DATETIME created"`
}
