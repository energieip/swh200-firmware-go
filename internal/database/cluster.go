package database

import (
	dl "github.com/energieip/common-components-go/pkg/dled"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
)

func GetClusterConfig(db Database) []sd.SwitchCluster {
	var cluster []sd.SwitchCluster
	stored, err := db.FetchAllRecords(dl.DbConfig, TableCluster)
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

func RemoveClusterConfig(db Database, cluster string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cluster
	return db.DeleteRecord(dl.DbConfig, TableCluster, criteria)
}

func UpdateClusterConfig(db Database, cluster map[string]sd.SwitchCluster) error {
	var res error
	for name, elt := range cluster {
		criteria := make(map[string]interface{})
		criteria["Mac"] = name
		var err error
		dbID := GetObjectID(db, dl.DbConfig, TableCluster, criteria)
		if dbID == "" {
			_, err = db.InsertRecord(dl.DbConfig, TableCluster, elt)
		} else {
			err = db.UpdateRecord(dl.DbConfig, TableCluster, dbID, elt)
		}
		if err != nil {
			res = err
		}
	}
	return res
}
