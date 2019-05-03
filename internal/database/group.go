package database

import (
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	dl "github.com/energieip/common-components-go/pkg/dled"
	"github.com/romana/rlog"
)

func GetStatusGroup(db Database) map[int]gm.GroupStatus {
	groups := make(map[int]gm.GroupStatus)
	stored, err := db.FetchAllRecords(gm.DbStatusName, gm.TableStatusName)
	if err != nil || stored == nil {
		return groups
	}
	for _, v := range stored {
		group, err := gm.ToGroupStatus(v)
		if err != nil {
			continue
		}
		groups[group.Group] = *group

	}
	return groups
}

func UpdateGroupStatus(db Database, status gm.GroupStatus) error {
	var err error
	criteria := make(map[string]interface{})
	criteria["Group"] = status.Group
	dbID := GetObjectID(db, gm.DbStatusName, gm.TableStatusName, criteria)
	if dbID == "" {
		_, err = db.InsertRecord(gm.DbStatusName, gm.TableStatusName, status)
	} else {
		err = db.UpdateRecord(gm.DbStatusName, gm.TableStatusName, dbID, status)
	}
	return err
}

func UpdateGroupConfig(db Database, cfg gm.GroupConfig) error {
	var err error
	criteria := make(map[string]interface{})
	criteria["Group"] = cfg.Group
	dbID := GetObjectID(db, dl.DbConfig, gm.TableStatusName, criteria)
	if dbID == "" {
		_, err = db.InsertRecord(dl.DbConfig, gm.TableStatusName, cfg)
	} else {
		err = db.UpdateRecord(dl.DbConfig, gm.TableStatusName, dbID, cfg)
	}
	return err
}

func GetGroupsConfig(db Database) map[int]gm.GroupConfig {
	groups := make(map[int]gm.GroupConfig)
	stored, err := db.FetchAllRecords(dl.DbConfig, gm.TableStatusName)
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
