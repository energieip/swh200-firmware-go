package core

import (
	"github.com/energieip/common-components-go/pkg/database"
	"github.com/energieip/common-components-go/pkg/dblind"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/romana/rlog"
)

type Database = database.DatabaseInterface

const (
	TableCluster = "clusters"
)

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
			tableCfg[dblind.TableName] = dblind.BlindSetup{}
			tableCfg[TableCluster] = pkg.Broker{}
		} else {
			tableCfg[dl.TableName] = dl.Led{}
			tableCfg[ds.TableName] = ds.Sensor{}
			tableCfg[gm.TableStatusName] = gm.GroupStatus{}
			tableCfg[dblind.TableName] = dblind.Blind{}
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

func (s *Service) resetDB() error {
	var res error
	for _, dbName := range []string{dl.DbConfig, dl.DbStatus} {
		err := s.db.CreateDB(dbName)
		if err != nil {
			rlog.Warn("Create DB ", err.Error())
			res = err
		}

		tableCfg := make(map[string]interface{})
		if dbName == dl.DbConfig {
			tableCfg[dl.TableName] = dl.LedSetup{}
			tableCfg[ds.TableName] = ds.SensorSetup{}
			tableCfg[gm.TableStatusName] = gm.GroupConfig{}
			tableCfg[dblind.TableName] = dblind.BlindSetup{}
			tableCfg[TableCluster] = sd.SwitchCluster{}
		} else {
			tableCfg[dl.TableName] = dl.Led{}
			tableCfg[ds.TableName] = ds.Sensor{}
			tableCfg[gm.TableStatusName] = gm.GroupStatus{}
			tableCfg[dblind.TableName] = dblind.Blind{}
		}
		for tableName, objs := range tableCfg {
			err = s.db.DropTable(dbName, tableName)
			if err != nil {
				rlog.Warn("Cannot drop table ", err.Error())
				continue
			}
			err = s.db.CreateTable(dbName, tableName, &objs)
			if err != nil {
				rlog.Warn("Create table ", err.Error())
				res = err
			}
		}
	}
	return res
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

func (s *Service) getStatusBlinds() map[string]dblind.Blind {
	drivers := make(map[string]dblind.Blind)
	stored, err := s.db.FetchAllRecords(dblind.DbStatus, dblind.TableName)
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

func (s *Service) getConfigBlind(mac string) *dblind.BlindSetup {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(dblind.DbConfig, dblind.TableName, criteria)
	if err != nil || stored == nil {
		return nil
	}
	driver, err := dblind.ToBlindSetup(stored)
	if err != nil {
		return nil
	}
	return driver
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

func (s *Service) updateGroupConfig(cfg gm.GroupConfig) error {
	var err error
	criteria := make(map[string]interface{})
	criteria["Group"] = cfg.Group
	dbID := s.getObjectID(dl.DbConfig, gm.TableStatusName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(dl.DbConfig, gm.TableStatusName, cfg)
	} else {
		err = s.db.UpdateRecord(dl.DbConfig, gm.TableStatusName, dbID, cfg)
	}
	return err
}

func (s *Service) getGroupsConfig() map[int]gm.GroupConfig {
	groups := make(map[int]gm.GroupConfig)
	stored, err := s.db.FetchAllRecords(dl.DbConfig, gm.TableStatusName)
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

func (s *Service) updateClusterConfig(cluster map[string]sd.SwitchCluster) error {
	var res error
	for name, elt := range cluster {
		criteria := make(map[string]interface{})
		criteria["Mac"] = name
		var err error
		dbID := s.getObjectID(dl.DbConfig, TableCluster, criteria)
		if dbID == "" {
			_, err = s.db.InsertRecord(dl.DbConfig, TableCluster, elt)
		} else {
			err = s.db.UpdateRecord(dl.DbConfig, TableCluster, dbID, elt)
		}
		if err != nil {
			res = err
		}
	}
	return res
}

func (s *Service) getClusterConfig() []sd.SwitchCluster {
	var cluster []sd.SwitchCluster
	stored, err := s.db.FetchAllRecords(dl.DbConfig, TableCluster)
	if err != nil || stored == nil {
		return cluster
	}
	for _, v := range stored {
		cl, err := sd.ToSwitchCluster(v)
		if err != nil {
			continue
		}
		cluster = append(cluster, *cl)

	}
	return cluster
}

func (s *Service) removeClusterConfig(cluster string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cluster
	return s.db.DeleteRecord(dl.DbConfig, TableCluster, criteria)
}
