package models

import (
	"time"
)

type BzKafkaInfo struct {
	Id          string    `xorm:"not null pk comment('主健') VARCHAR(50)"`
	KaTopic     string    `xorm:"not null comment('kafka topic') unique VARCHAR(32)"`
	KaBroker    string    `xorm:"not null comment('kafka broker') VARCHAR(256)"`
	KaWClientid string    `xorm:"default '' comment('生产者clientid') VARCHAR(32)"`
	KaWUser     string    `xorm:"comment('生产者用户') VARCHAR(32)"`
	KaWPwd      string    `xorm:"comment('生产者密码') VARCHAR(32)"`
	KaRClientid string    `xorm:"default '' comment('消费者clientid') VARCHAR(32)"`
	KaRUser     string    `xorm:"comment('消费者用户') VARCHAR(32)"`
	KaRPwd      string    `xorm:"comment('消费者密码') VARCHAR(32)"`
	KaGroup     string    `xorm:"comment('消费者组ID') VARCHAR(32)"`
	KaStatus    int32     `xorm:"default 1 comment('状态 0-下线 1-正常') TINYINT(1)"`
	KaVer       string    `xorm:"comment('kafka版本') VARCHAR(16)"`
	KaDesc      string    `xorm:"comment('kafka描述') VARCHAR(256)"`
	CreatedAt   time.Time `xorm:"DATETIME created"`
	UpdatedAt   time.Time `xorm:"DATETIME updated"`
}
