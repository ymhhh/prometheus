package models

type BzMapTemplatefarel2alert struct {
	Id                string `xorm:"not null pk default '' VARCHAR(50)"`
	PxTemplateFaRelId int64  `xorm:"not null default 0 BIGINT(20)"`
	BzAlertId         string `xorm:"not null default '' VARCHAR(50)"`
}
