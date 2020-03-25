package models

type BzActionType struct {
	Id         int32  `xorm:"not null pk autoincr INT(11)"`
	Name       string `xorm:"not null default '' VARCHAR(50)"`
	Decription string `xorm:"not null default '' VARCHAR(100)"`
}
