package models

import (
	"time"
)

type BzServiceExt struct {
	Id        string    `xorm:"not null pk comment('服务ID') VARCHAR(50)"`
	DataFmt   int32     `xorm:"not null comment('请求数据格式 1-opentsdb 2-jmx 3-自定义 4-phenix 5-flink 7-prom 8-relay 9-trans') TINYINT(1)"`
	KaTopic   string    `xorm:"not null comment('kafka topic') VARCHAR(32)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
	UpdatedAt time.Time `xorm:"DATETIME updated"`
}
