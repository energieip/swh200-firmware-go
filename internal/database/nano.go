package database

import (
	dn "github.com/energieip/common-components-go/pkg/dnanosense"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func RemoveNanoStatus(db Database, label string) error {
	criteria := make(map[string]interface{})
	criteria["Label"] = label
	return db.DeleteRecord(pconst.DbStatus, pconst.TbNanosenses, criteria)
}

func GetStatusNanos(db Database) map[string]dn.Nanosense {
	nanos := make(map[string]dn.Nanosense)
	stored, err := db.FetchAllRecords(pconst.DbStatus, pconst.TbNanosenses)
	if err != nil || stored == nil {
		return nanos
	}
	for _, v := range stored {
		cell, err := dn.ToNanosense(v)
		if err != nil || cell == nil {
			continue
		}
		nanos[cell.Mac] = *cell
	}
	return nanos
}

//SaveNanoStatus dump sensor status in database
func SaveNanoStatus(db Database, cfg dn.Nanosense) error {
	criteria := make(map[string]interface{})
	criteria["Label"] = cfg.Label
	return SaveOnUpdateObject(db, cfg, pconst.DbStatus, pconst.TbNanosenses, criteria)
}
