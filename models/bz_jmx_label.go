package models

import (
	"time"
)

type BzJmxLabel struct {
	Id          string    `xorm:"not null pk comment('主健') VARCHAR(50)"`
	LabelName   string    `xorm:"comment('签名的名称') unique(bz_jmx_label_lname_nameid_uidx) VARCHAR(128)"`
	NameId      string    `xorm:"not null comment('name id') unique(bz_jmx_label_lname_nameid_uidx) VARCHAR(50)"`
	LabelStatus int       `xorm:"default 1 comment('状态 0-下线 1-正常') TINYINT(1)"`
	CreatedAt   time.Time `xorm:"not null comment('添加时间') DATETIME"`
	UpdatedAt   time.Time `xorm:"not null comment('更新时间') DATETIME"`
}
