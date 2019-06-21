package database

import (
	"github.com/energieip/common-components-go/pkg/dhvac"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func RemoveHvacStatus(db Database, mac string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	return db.DeleteRecord(pconst.DbStatus, pconst.TbHvacs, criteria)
}

func GetStatusHvacs(db Database) map[string]dhvac.Hvac {
	drivers := make(map[string]dhvac.Hvac)
	stored, err := db.FetchAllRecords(pconst.DbStatus, pconst.TbHvacs)
	if err != nil || stored == nil {
		return drivers
	}
	for _, v := range stored {
		driver, err := dhvac.ToHvac(v)
		if err != nil {
			continue
		}
		drivers[driver.Mac] = *driver
	}
	return drivers
}

func GetConfigHvac(db Database, mac string) *dhvac.HvacSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbHvacs, criteria)
	if err != nil || stored == nil {
		return nil
	}
	driver, err := dhvac.ToHvacSetup(stored)
	if err != nil {
		return nil
	}
	return driver
}

func GetHvacConfig(db Database, mac string) (*dhvac.HvacSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbHvacs, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dhvac.ToHvacSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}

//SaveHvacStatus dump hvac status in database
func SaveHvacStatus(db Database, cfg dhvac.Hvac) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbStatus, pconst.TbHvacs, criteria)
}

//SaveHvacConfig dump hvac config in database
func SaveHvacConfig(db Database, cfg dhvac.HvacConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbHvacs, criteria)
}

//SaveHvacSetup dump hvac config in database
func SaveHvacSetup(db Database, cfg dhvac.HvacSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbHvacs, criteria)
}
