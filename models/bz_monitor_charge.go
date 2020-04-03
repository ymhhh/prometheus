package models

import (
	"time"
)

type BzMonitorCharge struct {
	Id        string    `xorm:"not null pk default '' VARCHAR(50)"`
	MonitorId string    `xorm:"not null default '' index VARCHAR(50)"`
	Erp       string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"not null default 'CURRENT_TIMESTAMP' DATETIME"`
}
