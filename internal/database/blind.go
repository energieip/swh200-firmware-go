package database

import (
	"github.com/energieip/common-components-go/pkg/dblind"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetBlindConfig(db Database, mac string) (*dblind.BlindSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbBlinds, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dblind.ToBlindSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}

func RemoveBlindStatus(db Database, mac string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	return db.DeleteRecord(pconst.DbStatus, pconst.TbBlinds, criteria)
}

func GetConfigBlind(db Database, mac string) *dblind.BlindSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbBlinds, criteria)
	if err != nil || stored == nil {
		return nil
	}
	driver, err := dblind.ToBlindSetup(stored)
	if err != nil {
		return nil
	}
	return driver
}

func GetStatusBlinds(db Database) map[string]dblind.Blind {
	drivers := make(map[string]dblind.Blind)
	stored, err := db.FetchAllRecords(pconst.DbStatus, pconst.TbBlinds)
	if err != nil || stored == nil {
		return drivers
	}
	for _, v := range stored {
		driver, err := dblind.ToBlind(v)
		if err != nil {
			continue
		}
		drivers[driver.Mac] = *driver
	}
	return drivers
}

//SaveBlindStatus dump blind status in database
func SaveBlindStatus(db Database, cfg dblind.Blind) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbStatus, pconst.TbBlinds, criteria)
}

//SaveBlindConfig dump blind config in database
func SaveBlindConfig(db Database, cfg dblind.BlindConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbBlinds, criteria)
}

//SaveBlindSetup dump blind config in database
func SaveBlindSetup(db Database, cfg dblind.BlindSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbBlinds, criteria)
}
