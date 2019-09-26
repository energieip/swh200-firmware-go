package database

import (
	dl "github.com/energieip/common-components-go/pkg/dled"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetConfigLed(db Database, mac string) *dl.LedSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbLeds, criteria)
	if err != nil || stored == nil {
		return nil
	}
	light, err := dl.ToLedSetup(stored)
	if err != nil {
		return nil
	}
	return light
}

func GetLedConfig(db Database, mac string) (*dl.LedSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbLeds, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dl.ToLedSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}

func GetLedsConfig(db Database) map[string]dl.LedSetup {
	leds := make(map[string]dl.LedSetup)
	stored, err := db.FetchAllRecords(pconst.DbConfig, pconst.TbLeds)
	if err != nil || stored == nil {
		return leds
	}
	for _, v := range stored {
		dr, err := dl.ToLedSetup(v)
		if err != nil || dr == nil {
			continue
		}
		leds[dr.Mac] = *dr
	}
	return leds
}

//SaveLedConfig dump led config in database
func SaveLedConfig(db Database, cfg dl.LedConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbLeds, criteria)
}

//SaveLedSetup dump led config in database
func SaveLedSetup(db Database, cfg dl.LedSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbLeds, criteria)
}
