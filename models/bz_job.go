package models

import (
	"time"
)

type BzJob struct {
	Id             string    `xorm:"not null pk VARCHAR(45)"`
	Name           string    `xorm:"VARCHAR(45)"`
	NameCn         string    `xorm:"VARCHAR(45)"`
	Owner          string    `xorm:"VARCHAR(50)"`
	Members        string    `xorm:"VARCHAR(45)"`
	CreatedUser    string    `xorm:"VARCHAR(45)"`
	UpdatedUser    string    `xorm:"VARCHAR(45)"`
	CreatedAt      time.Time `xorm:"DATETIME created"`
	UpdatedAt      time.Time `xorm:"DATETIME updated"`
	Deleted        string    `xorm:"VARCHAR(45)"`
	DepartmentCode string    `xorm:"VARCHAR(45)"`
	DepartmentName string    `xorm:"VARCHAR(45)"`
	Type           string    `xorm:"VARCHAR(45)"`
	SystemId       string    `xorm:"VARCHAR(45)"`
	ParentJobId    string    `xorm:"VARCHAR(45)"`
	FullPath       string    `xorm:"VARCHAR(45)"`
	Version        int       `xorm:"version"`
}
