// Copyright 2020 The BDP
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// added by huanghonghu

package mysql

import (
	"strings"

	"github.com/prometheus/common/model"
)

// TKmServers 鲲鹏机器
type TKmServers struct {
	IP                 string           `xorm:"'IP' not null VARCHAR(24)"`
	HostName           string           `xorm:"'HOST_NAME' VARCHAR(60)"`
	ProductLine        string           `xorm:"'PRODUCT_LINE' VARCHAR(12)"`
	MachineRoom        model.LabelValue `xorm:"'MACHINE_ROOM' VARCHAR(100)"`
	UseStatus          model.LabelValue `xorm:"'USE_STATUS' VARCHAR(100)"`
	Cabinet            model.LabelValue `xorm:"'CABINET' VARCHAR(100)"`
	Fzr                model.LabelValue `xorm:"'FZR' VARCHAR(100)"`
	Contact            string           `xorm:"'CONTACT' CHAR(255)"`
	CPU                string           `xorm:"'CPU' VARCHAR(100)"`
	Memory             string           `xorm:"'MEMORY' VARCHAR(100)"`
	Disk               string           `xorm:"'DISK' VARCHAR(100)"`
	Vendor             model.LabelValue `xorm:"'VENDOR' VARCHAR(100)"`
	CreateTime         int64            `xorm:"'CREATE_TIME' BIGINT(20)"`
	OnlineTime         int64            `xorm:"'ONLINE_TIME' BIGINT(20)"`
	WarrantyTime       int64            `xorm:"'WARRANTY_TIME' BIGINT(20)"`
	CreateDate         string           `xorm:"'CREATE_DATE' not null VARCHAR(20)"`
	ClusterName        model.LabelValue `xorm:"'CLUSTER_NAME' VARCHAR(500)"`
	DeploymentServices model.LabelValue `xorm:"'DEPLOYMENT_SERVICES' VARCHAR(100)"`
	Tag                string           `xorm:"VARCHAR(256)"`
	Model              model.LabelValue `xorm:"VARCHAR(200)"`
	Attribution        model.LabelValue `xorm:"VARCHAR(200)"`

	tagMaps model.LabelSet `xorm:"-"`
}

func (p *TKmServers) parseTag() {
	p.tagMaps = make(model.LabelSet)
	if len(p.Tag) == 0 {
		return
	}
	labels := make(map[model.LabelName][]string)

	for _, tag := range strings.Split(p.Tag, ",") {
		kvs := strings.Split(tag, ":")
		if len(kvs) < 2 {
			continue
		}
		labelKey := model.LabelName(kvs[0])
		labels[labelKey] = append(labels[labelKey], kvs[1])
	}

	for key, lSlice := range labels {
		p.tagMaps[key] = model.LabelValue(strings.Join(lSlice, ","))
	}
}
