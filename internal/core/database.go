package core

import (
	"github.com/energieip/common-components-go/pkg/database"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	"github.com/romana/rlog"
)

type Database = database.DatabaseInterface

func (s *Service) connectDatabase(ip, port string) error {
	db, err := database.NewDatabase(database.RETHINKDB)
	if err != nil {
		rlog.Error("database err " + err.Error())
		return err
	}
	s.db = db

	confDb := database.DatabaseConfig{
		IP:   ip,
		Port: port,
	}
	err = s.db.Initialize(confDb)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return err
	}

	for _, dbName := range []string{dl.DbConfig, dl.DbStatus} {
		err = db.CreateDB(dbName)
		if err != nil {
			rlog.Warn("Create DB ", err.Error())
		}

		tableCfg := make(map[string]interface{})
		if dbName == dl.DbConfig {
			tableCfg[dl.TableName] = dl.LedSetup{}
			tableCfg[ds.TableName] = ds.SensorSetup{}
			tableCfg[gm.TableStatusName] = gm.GroupConfig{}
		} else {
			tableCfg[dl.TableName] = dl.Led{}
			tableCfg[ds.TableName] = ds.Sensor{}
			tableCfg[gm.TableStatusName] = gm.GroupStatus{}
		}
		for tableName, objs := range tableCfg {
			err = db.CreateTable(dbName, tableName, &objs)
			if err != nil {
				rlog.Warn("Create table ", err.Error())
			}
		}
	}

	return nil
}

func (s *Service) dbClose() error {
	return s.db.Close()
}

func (s *Service) getObjectID(dbName, tbName string, criteria map[string]interface{}) string {
	stored, err := s.db.GetRecord(dbName, tbName, criteria)
	if err == nil && stored != nil {
		m := stored.(map[string]interface{})
		id, ok := m["id"]
		if ok {
			return id.(string)
		}
	}
	return ""
}

func (s *Service) getStatusLeds() map[string]dl.Led {
	leds := make(map[string]dl.Led)
	stored, err := s.db.FetchAllRecords(dl.DbStatus, dl.TableName)
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

func (s *Service) getStatusSensors() map[string]ds.Sensor {
	sensors := make(map[string]ds.Sensor)
	stored, err := s.db.FetchAllRecords(ds.DbStatus, ds.TableName)
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

func (s *Service) getSensorStatus(mac string) *ds.Sensor {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	sensorStored, err := s.db.GetRecord(ds.DbStatus, ds.TableName, criteria)
	if err != nil || sensorStored == nil {
		return nil
	}
	cell, err := ds.ToSensor(sensorStored)
	if err != nil {
		return nil
	}
	return cell
}

func (s *Service) getStatusLed(mac string) *dl.Led {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	ledStored, err := s.db.GetRecord(dl.DbStatus, dl.TableName, criteria)
	if err != nil || ledStored == nil {
		return nil
	}
	light, err := dl.ToLed(ledStored)
	if err != nil {
		return nil
	}
	return light
}

func (s *Service) getConfigLed(mac string) *dl.LedSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(dl.DbConfig, dl.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	light, err := dl.ToLedSetup(stored)
	if err != nil {
		return nil
	}
	return light
}

func (s *Service) getConfigSensor(mac string) *ds.SensorSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(ds.DbConfig, ds.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	sensor, err := ds.ToSensorSetup(stored)
	if err != nil {
		return nil
	}
	return sensor
}

func (s *Service) getStatusGroup() map[int]gm.GroupStatus {
	groups := make(map[int]gm.GroupStatus)
	stored, err := s.db.FetchAllRecords(gm.DbStatusName, gm.TableStatusName)
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

func (s *Service) updateGroupStatus(status gm.GroupStatus) error {
	var err error
	criteria := make(map[string]interface{})
	criteria["Group"] = status.Group
	dbID := s.getObjectID(gm.DbStatusName, gm.TableStatusName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(gm.DbStatusName, gm.TableStatusName, status)
	} else {
		err = s.db.UpdateRecord(gm.DbStatusName, gm.TableStatusName, dbID, status)
	}
	return err
}
