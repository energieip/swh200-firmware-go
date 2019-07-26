package database

import (
	"github.com/energieip/common-components-go/pkg/dwago"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetWagoConfig(db Database, mac string) (*dwago.WagoDef, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbWagos, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dwago.ToWagoDef(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}

//SaveWagoConfig dump wago config in database
func SaveWagoConfig(db Database, cfg dwago.WagoDef) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = cfg.Mac
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbWagos, criteria)
}

func GetWagosConfig(db Database) map[string]dwago.WagoDef {
	wagos := make(map[string]dwago.WagoDef)
	stored, err := db.FetchAllRecords(pconst.DbConfig, pconst.TbWagos)
	if err != nil || stored == nil {
		return wagos
	}
	for _, v := range stored {
		gr, err := dwago.ToWagoDef(v)
		if err != nil || gr == nil {
			continue
		}
		wagos[gr.Mac] = *gr
	}
	return wagos
}

func RemoveWagoStatus(db Database, mac string) error {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	return db.DeleteRecord(pconst.DbStatus, pconst.TbWagos, criteria)
}

func GetStatusWagos(db Database) map[string]dwago.Wago {
	wagos := make(map[string]dwago.Wago)
	stored, err := db.FetchAllRecords(pconst.DbStatus, pconst.TbWagos)
	if err != nil || stored == nil {
		return wagos
	}
	for _, v := range stored {
		cell, err := dwago.ToWago(v)
		if err != nil || cell == nil {
			continue
		}
		wagos[cell.Mac] = *cell
	}
	return wagos
}

//SaveWagoStatus dump wago status in database
func SaveWagoStatus(db Database, cfg dwago.Wago) error {
	criteria := make(map[string]interface{})
	criteria["Label"] = cfg.Label
	return SaveOnUpdateObject(db, cfg, pconst.DbStatus, pconst.TbWagos, criteria)
}
