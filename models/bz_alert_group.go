package models

import (
	"time"
)

type BzAlertGroup struct {
	Id          string    `xorm:"not null pk default '' VARCHAR(50)"`
	Name        string    `xorm:"not null default '' VARCHAR(100)"`
	CreatorErp  string    `xorm:"not null default '' comment('创建人ERP') VARCHAR(50)"`
	Description string    `xorm:"VARCHAR(100)"`
	CreatedAt   time.Time `xorm:"DATETIME created"`
}
