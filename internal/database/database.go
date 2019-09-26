package database

import (
	"github.com/energieip/common-components-go/pkg/database"
	"github.com/energieip/common-components-go/pkg/dblind"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/dhvac"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	"github.com/energieip/common-components-go/pkg/duser"
	"github.com/energieip/common-components-go/pkg/dwago"
	"github.com/energieip/common-components-go/pkg/pconst"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/romana/rlog"
)

type Database = database.DatabaseInterface

const (
	TableCluster = "clusters"
	AccessTable  = "access"
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
	prepareDB(db, false)
	return db, nil
}

func prepareDB(db Database, withDrop bool) {
	for _, dbName := range []string{pconst.DbConfig, pconst.DbStatus} {
		err := db.CreateDB(dbName)
		if err != nil {
			rlog.Warn("Create DB ", err.Error())
		}

		tableCfg := make(map[string]interface{})
		if dbName == pconst.DbConfig {
			tableCfg[pconst.TbLeds] = dl.LedSetup{}
			tableCfg[pconst.TbSensors] = ds.SensorSetup{}
			tableCfg[pconst.TbGroups] = gm.GroupConfig{}
			tableCfg[pconst.TbBlinds] = dblind.BlindSetup{}
			tableCfg[pconst.TbWagos] = dwago.WagoDef{}
			tableCfg[TableCluster] = pkg.Broker{}
			tableCfg[pconst.TbHvacs] = dhvac.HvacSetup{}
			tableCfg[AccessTable] = duser.UserAccess{}
			tableCfg[pconst.TbSwitchs] = sd.SwitchDefinition{}
		}
		for tableName, objs := range tableCfg {
			if withDrop {
				err = db.DropTable(dbName, tableName)
				if err != nil {
					rlog.Warn("Cannot drop table ", err.Error())
					continue
				}
			}
			err = db.CreateTable(dbName, tableName, &objs)
			if err != nil {
				rlog.Warn("Create table ", err.Error())
			}
		}
	}

}

func ResetDB(db Database) error {
	prepareDB(db, true)
	var res error
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
