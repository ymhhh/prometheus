package models

import (
	"time"
)

type BzCode struct {
	Id         string    `xorm:"not null pk VARCHAR(50)"`
	Name       string    `xorm:"not null default '' VARCHAR(50)"`
	NameCn     string    `xorm:"VARCHAR(50)"`
	CreatorErp string    `xorm:"VARCHAR(45)"`
	UpdaterErp string    `xorm:"VARCHAR(45)"`
	CreatedAt  time.Time `xorm:"DATETIME created"`
	UpdatedAt  time.Time `xorm:"DATETIME updated"`
	Deleted    int32     `xorm:"INT(11)"`
	Type       int32     `xorm:"INT(11)"`
	Git        string    `xorm:"VARCHAR(150)"`
	Desc       string    `xorm:"VARCHAR(150)"`
	ServiceId  int32     `xorm:"INT(11)"`
}
