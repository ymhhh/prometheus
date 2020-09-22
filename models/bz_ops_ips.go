package models

type BzOpsIps struct {
	IpNum    int64  `xorm:"not null pk default 0 comment('IP 转换为int') BIGINT(20)"`
	TagKey   string `xorm:"not null pk default '' comment('标签key') VARCHAR(100)"`
	TagValue string `xorm:"not null pk default '' comment('标签值') VARCHAR(200)"`
}
