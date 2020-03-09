package models

type BzMapTemplate2monitor struct {
	Id           string `xorm:"not null pk default '' VARCHAR(50)"`
	PxTemplateId int64  `xorm:"not null default 0 BIGINT(20)"`
	BzMonitorId  string `xorm:"not null default '' VARCHAR(50)"`
}
