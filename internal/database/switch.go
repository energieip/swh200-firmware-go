package database

import (
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	"github.com/energieip/common-components-go/pkg/pconst"
)

func GetSwitchConfig(db Database) sd.SwitchDefinition {
	var sw sd.SwitchDefinition
	stored, err := db.FetchAllRecords(pconst.DbConfig, pconst.TbSwitchs)
	if err != nil || stored == nil {
		return sw
	}
	for _, v := range stored {
		cl, err := sd.ToSwitchDefinition(v)
		if err != nil {
			continue
		}
		sw = *cl

	}
	return sw
}

func UpdateSwitchConfig(db Database, elt sd.SwitchDefinition) error {
	criteria := make(map[string]interface{})
	return SaveOnUpdateObject(db, elt, pconst.DbConfig, pconst.TbSwitchs, criteria)
}
