package database

import (
	"github.com/energieip/common-components-go/pkg/database"
	"github.com/energieip/common-components-go/pkg/dblind"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/dhvac"
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

//ConnectDatabase
func ConnectDatabase(ip, port string) (Database, error) {
	db, err := database.NewDatabase(database.RETHINKDB)
	if err != nil {
		rlog.Error("database err " + err.Error())
		return nil, err
	}

	confDb := database.DatabaseConfig{
		IP:   ip,
		Port: port,
	}
	err = db.Initialize(confDb)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return nil, err
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
			tableCfg[dhvac.TableName] = dhvac.HvacSetup{}
		} else {
			tableCfg[dl.TableName] = dl.Led{}
			tableCfg[ds.TableName] = ds.Sensor{}
			tableCfg[gm.TableStatusName] = gm.GroupStatus{}
			tableCfg[dblind.TableName] = dblind.Blind{}
			tableCfg[dhvac.TableName] = dhvac.Hvac{}
		}
		for tableName, objs := range tableCfg {
			err = db.CreateTable(dbName, tableName, &objs)
			if err != nil {
				rlog.Warn("Create table ", err.Error())
			}
		}
	}

	return db, nil
}

func ResetDB(db Database) error {
	var res error
	for _, dbName := range []string{dl.DbConfig, dl.DbStatus} {
		err := db.CreateDB(dbName)
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
			tableCfg[dhvac.TableName] = dhvac.HvacSetup{}
			tableCfg[TableCluster] = sd.SwitchCluster{}
		} else {
			tableCfg[dl.TableName] = dl.Led{}
			tableCfg[ds.TableName] = ds.Sensor{}
			tableCfg[gm.TableStatusName] = gm.GroupStatus{}
			tableCfg[dblind.TableName] = dblind.Blind{}
			tableCfg[dhvac.TableName] = dhvac.Hvac{}
		}
		for tableName, objs := range tableCfg {
			err = db.DropTable(dbName, tableName)
			if err != nil {
				rlog.Warn("Cannot drop table ", err.Error())
				continue
			}
			err = db.CreateTable(dbName, tableName, &objs)
			if err != nil {
				rlog.Warn("Create table ", err.Error())
				res = err
			}
		}
	}
	return res
}

func DBClose(db Database) error {
	return db.Close()
}

func GetObjectID(db Database, dbName, tbName string, criteria map[string]interface{}) string {
	stored, err := db.GetRecord(dbName, tbName, criteria)
	if err == nil && stored != nil {
		m := stored.(map[string]interface{})
		id, ok := m["id"]
		if ok {
			return id.(string)
		}
	}
	return ""
}

//SaveOnUpdateObject in database
func SaveOnUpdateObject(db Database, obj interface{}, dbName, tbName string, criteria map[string]interface{}) error {
	var err error
	dbID := GetObjectID(db, dbName, tbName, criteria)
	if dbID == "" {
		_, err = db.InsertRecord(dbName, tbName, obj)
	} else {
		err = db.UpdateRecord(dbName, tbName, dbID, obj)
	}
	return err
}
