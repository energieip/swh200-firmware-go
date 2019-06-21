package database

import (
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetClusterConfig(db Database) []sd.SwitchCluster {
	var cluster []sd.SwitchCluster
	stored, err := db.FetchAllRecords(pconst.DbConfig, TableCluster)
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
	return db.DeleteRecord(pconst.DbConfig, TableCluster, criteria)
}

func UpdateClusterConfig(db Database, cluster map[string]sd.SwitchCluster) error {
	var res error
	for name, elt := range cluster {
		criteria := make(map[string]interface{})
		criteria["Mac"] = name
		err := SaveOnUpdateObject(db, elt, pconst.DbConfig, TableCluster, criteria)
		if err != nil {
			res = err
		}
	}
	return res
}
