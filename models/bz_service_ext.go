package models

import (
	"time"
)

type BzServiceExt struct {
	Id        string    `xorm:"not null pk comment('服务ID') VARCHAR(50)"`
	DataFmt   int       `xorm:"not null comment('请求数据格式 1-opentsdb 2-jmx 3-自定义') TINYINT(1)"`
	KaTopic   string    `xorm:"not null comment('kafka topic') VARCHAR(32)"`
	CreatedAt time.Time `xorm:"not null comment('添加时间') DATETIME"`
	UpdatedAt time.Time `xorm:"not null comment('更新时间') DATETIME"`
}
