package models

type BzAlertThreshold struct {
	Id           string  `xorm:"not null pk VARCHAR(50)"`
	AlertId      string  `xorm:"not null index VARCHAR(50)"`
	Threshold    float64 `xorm:"DOUBLE"`
	ThresholdMax float64 `xorm:"DOUBLE"`
	For          string  `xorm:"VARCHAR(50)"`
	Severity     string  `xorm:"not null default 'normal' comment('报警等级：normal普通，warn告警，emergent紧急，critical严重') VARCHAR(50)"`
}
