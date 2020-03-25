package models

import (
	"time"
)

type BzAlertAction struct {
	Id               string    `xorm:"not null pk default '' VARCHAR(50)"`
	AlertId          string    `xorm:"not null default '' index VARCHAR(50)"`
	ActionId         int32     `xorm:"not null default 0 index INT(11)"`
	AlertGroupId     string    `xorm:"not null default '' index VARCHAR(50)"`
	CreatorErp       string    `xorm:"not null default '' VARCHAR(50)"`
	IsUsing          int32     `xorm:"not null default 0 comment('是否启用') TINYINT(1)"`
	Title            string    `xorm:"VARCHAR(50)"`
	Content          string    `xorm:"VARCHAR(1500)"`
	CreatedAt        time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
	UpdatedAt        time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
	TopAlertActionId string    `xorm:"VARCHAR(50)"`
}
