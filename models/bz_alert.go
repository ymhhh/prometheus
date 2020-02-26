package models

import (
	"time"
)

type BzAlert struct {
	Id         string    `xorm:"not null pk default '' VARCHAR(50)"`
	MonitorId  string    `xorm:"not null default '' index VARCHAR(50)"`
	Name       string    `xorm:"not null default '' VARCHAR(50)"`
	CreatorErp string    `xorm:"not null default '' VARCHAR(50)"`
	Expression string    `xorm:"TEXT"`
	Status     string    `xorm:"not null default '' VARCHAR(50)"`
	Operator   string    `xorm:"comment('操作符') VARCHAR(45)"`
	IsUsing    int       `xorm:"not null default 0 TINYINT(1)"`
	CreatedAt  time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
	UpdatedAt  time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
	Version    int       `xorm:"INT(11)"`
}
