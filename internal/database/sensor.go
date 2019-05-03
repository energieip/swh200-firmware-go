package database

import ds "github.com/energieip/common-components-go/pkg/dsensor"

func GetSensorConfig(db Database, mac string) (*ds.SensorSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(ds.DbConfig, ds.TableName, criteria)
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

func RemoveSensorStatus(db Database, mac string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	return db.DeleteRecord(ds.DbStatus, ds.TableName, criteria)
}

func GetStatusSensors(db Database) map[string]ds.Sensor {
	sensors := make(map[string]ds.Sensor)
	stored, err := db.FetchAllRecords(ds.DbStatus, ds.TableName)
	if err != nil || stored == nil {
		return sensors
	}
	for _, v := range stored {
		cell, err := ds.ToSensor(v)
		if err != nil {
			continue
		}
		sensors[cell.Mac] = *cell
	}
	return sensors
}

func GetConfigSensor(db Database, mac string) *ds.SensorSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(ds.DbConfig, ds.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	sensor, err := ds.ToSensorSetup(stored)
	if err != nil {
		return nil
	}
	return sensor
}

//SaveSensorStatus dump sensor status in database
func SaveSensorStatus(db Database, cfg ds.Sensor) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, ds.DbStatus, ds.TableName, criteria)
}

//SaveSensorConfig dump sensor config in database
func SaveSensorConfig(db Database, cfg ds.SensorConf) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, ds.DbConfig, ds.TableName, criteria)
}

//SaveSensorSetup dump sensor config in database
func SaveSensorSetup(db Database, cfg ds.SensorSetup) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, ds.DbConfig, ds.TableName, criteria)
}
