package models

import (
	"time"
)

type BzLogAlert struct {
	Id                 int64     `xorm:"pk autoincr BIGINT(20)"`
	AlertId            string    `xorm:"not null default '' index VARCHAR(50)"`
	AlertTransactionId string    `xorm:"not null default '0' index VARCHAR(50)"`
	Expression         string    `xorm:"comment('监控表达式') TEXT"`
	Severity           string    `xorm:"not null default 'normal' comment('报警等级：0普通，1告警，2紧急') VARCHAR(50)"`
	Status             string    `xorm:"not null default '' VARCHAR(50)"`
	Summary            string    `xorm:"comment('发送标题') VARCHAR(150)"`
	Description        string    `xorm:"comment('发送的内容') VARCHAR(1500)"`
	CreatedAt          time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
}
