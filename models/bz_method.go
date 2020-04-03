package models

import (
	"time"
)

type BzMethod struct {
	Id        string    `xorm:"not null pk VARCHAR(50)"`
	Name      string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME"`
	UpdatedAt time.Time `xorm:"DATETIME"`
	Deleted   int       `xorm:"INT(11)"`
	Type      int       `xorm:"INT(11)"`
	ProgramId string    `xorm:"VARCHAR(50)"`
	StackPath string    `xorm:"VARCHAR(45)"`
}
