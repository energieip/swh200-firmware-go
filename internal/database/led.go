package database

import dl "github.com/energieip/common-components-go/pkg/dled"

func GetStatusLed(db Database, mac string) *dl.Led {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(dl.DbStatus, dl.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	light, err := dl.ToLed(stored)
	if err != nil {
		return nil
	}
	return light
}

func GetStatusLeds(db Database) map[string]dl.Led {
	leds := make(map[string]dl.Led)
	stored, err := db.FetchAllRecords(dl.DbStatus, dl.TableName)
	if err != nil || stored == nil {
		return leds
	}
	for _, v := range stored {
		light, err := dl.ToLed(v)
		if err != nil {
			continue
		}
		leds[light.Mac] = *light
	}
	return leds
}

func GetConfigLed(db Database, mac string) *dl.LedSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(dl.DbConfig, dl.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	light, err := dl.ToLedSetup(stored)
	if err != nil {
		return nil
	}
	return light
}

func RemoveLedStatus(db Database, mac string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	return db.DeleteRecord(dl.DbStatus, dl.TableName, criteria)
}

func GetLedConfig(db Database, mac string) (*dl.LedSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(dl.DbConfig, dl.TableName, criteria)
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

//SaveLedStatus dump led status in database
func SaveLedStatus(db Database, cfg dl.Led) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, dl.DbStatus, dl.TableName, criteria)
}

//SaveLedConfig dump led config in database
func SaveLedConfig(db Database, cfg dl.LedConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, dl.DbConfig, dl.TableName, criteria)
}

//SaveLedSetup dump led config in database
func SaveLedSetup(db Database, cfg dl.LedSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, dl.DbConfig, dl.TableName, criteria)
}
