package models

import (
	"time"
)

type BzJmxName struct {
	Id         string    `xorm:"not null pk comment('主健') VARCHAR(50)"`
	NameVal    string    `xorm:"not null comment('name值') unique(bz_jmx_name_prod_name_uidx) VARCHAR(128)"`
	ServiceId  string    `xorm:"not null unique(bz_jmx_name_prod_name_uidx) VARCHAR(50)"`
	NameStatus int       `xorm:"default 1 comment('状态 0-下线 1-正常') TINYINT(1)"`
	MatchRule  int       `xorm:"default 1 comment('匹配规则 1-完全匹配 2-正则匹配，完全匹配优先级高') TINYINT(1)"`
	NameDesc   string    `xorm:"comment('name描述') VARCHAR(256)"`
	CreatedAt  time.Time `xorm:"not null comment('添加时间') DATETIME"`
	UpdatedAt  time.Time `xorm:"not null comment('更新时间') DATETIME"`
}
