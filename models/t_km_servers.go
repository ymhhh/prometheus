package models

type TKmServers struct {
	Ip                 string `xorm:"not null VARCHAR(24)"`
	HostName           string `xorm:"VARCHAR(60)"`
	ProductLine        string `xorm:"VARCHAR(12)"`
	MachineRoom        string `xorm:"VARCHAR(100)"`
	UseStatus          string `xorm:"VARCHAR(100)"`
	Cabinet            string `xorm:"VARCHAR(100)"`
	Fzr                string `xorm:"VARCHAR(100)"`
	Contact            string `xorm:"CHAR(255)"`
	Cpu                string `xorm:"VARCHAR(100)"`
	Memory             string `xorm:"VARCHAR(100)"`
	Disk               string `xorm:"VARCHAR(100)"`
	Vendor             string `xorm:"VARCHAR(100)"`
	CreateTime         int64  `xorm:"BIGINT(20)"`
	OnlineTime         int64  `xorm:"BIGINT(20)"`
	WarrantyTime       int64  `xorm:"BIGINT(20)"`
	CreateDate         string `xorm:"not null VARCHAR(20)"`
	ClusterName        string `xorm:"VARCHAR(500)"`
	DeploymentServices string `xorm:"VARCHAR(100)"`
	Tag                string `xorm:"VARCHAR(1000)"`
	Model              string `xorm:"VARCHAR(200)"`
	Attribution        string `xorm:"VARCHAR(200)"`
}
