package models

type BzAction struct {
	Id         int    `xorm:"not null pk autoincr INT(11)"`
	ActionType int    `xorm:"not null default 0 index INT(11)"`
	Name       string `xorm:"not null VARCHAR(50)"`
}
