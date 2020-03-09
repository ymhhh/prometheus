package models

import (
	"time"
)

type BzAlertGroupErp struct {
	Id           string    `xorm:"not null pk default '' VARCHAR(50)"`
	AlertGroupId string    `xorm:"not null default '' comment('报警组Id') index VARCHAR(50)"`
	Erp          string    `xorm:"not null comment('ERP账号') VARCHAR(50)"`
	CreatedAt    time.Time `xorm:"default CURRENT_TIMESTAMP comment('创建时间') DATETIME"`
}
