package database

import (
	"github.com/energieip/common-components-go/pkg/dhvac"
	"github.com/energieip/common-components-go/pkg/pconst"
)

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
