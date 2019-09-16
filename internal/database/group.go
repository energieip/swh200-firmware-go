package database

import (
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/pconst"
	"github.com/romana/rlog"
)

func UpdateGroupConfig(db Database, cfg gm.GroupConfig) error {
	criteria := make(map[string]interface{})
	criteria["Group"] = cfg.Group
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbGroups, criteria)
}

func GetGroupsConfig(db Database) map[int]gm.GroupConfig {
	groups := make(map[int]gm.GroupConfig)
	stored, err := db.FetchAllRecords(pconst.DbConfig, pconst.TbGroups)
	rlog.Info("==== stored , err", stored, err)
	if err != nil || stored == nil {
		return groups
	}
	for _, v := range stored {
		gr, err := gm.ToGroupConfig(v)
		if err != nil || gr == nil {
			continue
		}
		groups[gr.Group] = *gr
	}
	return groups
}
