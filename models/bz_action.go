package models

type BzAction struct {
	Id         int32  `xorm:"not null pk autoincr INT(11)"`
	ActionType int32  `xorm:"not null default 0 index INT(11)"`
	Name       string `xorm:"not null VARCHAR(50)"`
}
