package models

import (
	"time"
)

type BzAlertProbe struct {
	Id         string    `xorm:"not null pk default '' VARCHAR(50)"`
	ServiceId  string    `xorm:"not null default '' index VARCHAR(50)"`
	Name       string    `xorm:"VARCHAR(100)"`
	Instances  string    `xorm:"TEXT"`
	Owner      string    `xorm:"TEXT"`
	Members    string    `xorm:"TEXT"`
	Labels     string    `xorm:"TEXT"`
	Tags       string    `xorm:"TEXT"`
	Type       string    `xorm:"not null default 'http' VARCHAR(10)"`
	AddLabels  bool      `xorm:"default '1' TINYINT(1)"`
	CreatorErp string    `xorm:"VARCHAR(50)"`
	UpdaterErp string    `xorm:"VARCHAR(50)"`
	CreatedAt  time.Time `xorm:"DATETIME created"`
	UpdatedAt  time.Time `xorm:"DATETIME updated"`
	Version    int       `xorm:"version"`
}
