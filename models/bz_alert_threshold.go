package models

type BzAlertThreshold struct {
	Id              string  `xorm:"not null pk VARCHAR(50)"`
	AlertId         string  `xorm:"not null index VARCHAR(50)"`
	Metric          string  `xorm:"not null default '' VARCHAR(200)"`
	Operator        string  `xorm:"not null default '' VARCHAR(20)"`
	Threshold       float64 `xorm:"DOUBLE"`
	ThresholdMax    float64 `xorm:"DOUBLE"`
	ThresholdType   int32   `xorm:"not null default 0 index INT(11)"`
	ComputeFunction string  `xorm:" VARCHAR(50)"`
	Offset          string  `xorm:" VARCHAR(11)"`
	RangeVectors    string  `xorm:" VARCHAR(11)"`
}
