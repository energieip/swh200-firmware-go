package database

import (
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetSensorConfig(db Database, mac string) (*ds.SensorSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbSensors, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := ds.ToSensorSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}

func GetConfigSensor(db Database, mac string) *ds.SensorSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbSensors, criteria)
	if err != nil || stored == nil {
		return nil
	}
	sensor, err := ds.ToSensorSetup(stored)
	if err != nil {
		return nil
	}
	return sensor
}

//SaveSensorConfig dump sensor config in database
func SaveSensorConfig(db Database, cfg ds.SensorConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbSensors, criteria)
}

//SaveSensorSetup dump sensor config in database
func SaveSensorSetup(db Database, cfg ds.SensorSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbSensors, criteria)
}
