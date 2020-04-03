package models

type BzApplication struct {
	Id         string `xorm:"not null pk VARCHAR(50)"`
	AppName    string `xorm:"VARCHAR(50)"`
	AppState   string `xorm:"VARCHAR(50)"`
	CreateTime string `xorm:"VARCHAR(50)"`
	StartTime  string `xorm:"VARCHAR(50)"`
	EndTime    string `xorm:"VARCHAR(50)"`
	Source     string `xorm:"VARCHAR(50)"`
	CreatorErp string `xorm:"VARCHAR(45)"`
	JobId      string `xorm:"VARCHAR(50)"`
	SystemId   string `xorm:"VARCHAR(45)"`
	Type       string `xorm:"VARCHAR(45)"`
	FullPath   string `xorm:"VARCHAR(45)"`
	Version    string `xorm:"VARCHAR(45)"`
}
