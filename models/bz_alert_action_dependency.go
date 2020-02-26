package models

type BzAlertActionDependency struct {
	Id             string `xorm:"not null pk default '' VARCHAR(50)"`
	AlertActionId  string `xorm:"not null default '' VARCHAR(50)"`
	ParentActionId string `xorm:"not null default '' VARCHAR(50)"`
}
