package database

import (
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/pconst"
)

//GetGroupConfig return the group configuration
func GetGroupConfig(db Database, grID int) (*gm.GroupConfig, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Group"] = grID
	stored, err := db.GetRecord(pconst.DbConfig, pconst.TbGroups, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	gr, err := gm.ToGroupConfig(stored)
	if err != nil {
		return nil, dbID
	}
	return gr, dbID
}

//UpdateGroupConfig update group config in database
func UpdateGroupConfig(db Database, config gm.GroupConfig) error {
	setup, dbID := GetGroupConfig(db, config.Group)
	if setup == nil || dbID == "" {
		criteria := make(map[string]interface{})
		criteria["Group"] = config.Group
		return SaveOnUpdateObject(db, config, pconst.DbConfig, pconst.TbGroups, criteria)
	}

	if config.Leds != nil {
		setup.Leds = config.Leds
	}

	if config.Sensors != nil {
		setup.Sensors = config.Sensors
	}

	if config.Blinds != nil {
		setup.Blinds = config.Blinds
	}

	if config.Hvacs != nil {
		setup.Hvacs = config.Hvacs
	}

	if config.Nanosenses != nil {
		setup.Nanosenses = config.Nanosenses
	}

	if config.FriendlyName != nil {
		setup.FriendlyName = config.FriendlyName
	}

	if config.CorrectionInterval != nil {
		setup.CorrectionInterval = config.CorrectionInterval
	}

	if config.Watchdog != nil {
		setup.Watchdog = config.Watchdog
	}

	if config.SlopeStartManual != nil {
		setup.SlopeStartManual = config.SlopeStartManual
	}

	if config.SlopeStopManual != nil {
		setup.SlopeStopManual = config.SlopeStopManual
	}

	if config.SlopeStartAuto != nil {
		setup.SlopeStartAuto = config.SlopeStartAuto
	}

	if config.SlopeStopAuto != nil {
		setup.SlopeStopAuto = config.SlopeStopAuto
	}

	if config.SensorRule != nil {
		setup.SensorRule = config.SensorRule
	}

	if config.Auto != nil {
		setup.Auto = config.Auto
	}

	if config.RuleBrightness != nil {
		setup.RuleBrightness = config.RuleBrightness
	}

	if config.RulePresence != nil {
		setup.RulePresence = config.RulePresence
	}

	if config.FirstDay != nil {
		setup.FirstDay = config.FirstDay
	}

	if config.FirstDayOffset != nil {
		setup.FirstDayOffset = config.FirstDayOffset
	}

	if config.SetpointOccupiedCool1 != nil {
		setup.SetpointOccupiedCool1 = config.SetpointOccupiedCool1
	}

	if config.SetpointOccupiedHeat1 != nil {
		setup.SetpointOccupiedHeat1 = config.SetpointOccupiedHeat1
	}

	if config.SetpointUnoccupiedCool1 != nil {
		setup.SetpointUnoccupiedCool1 = config.SetpointUnoccupiedCool1
	}

	if config.SetpointUnoccupiedHeat1 != nil {
		setup.SetpointUnoccupiedHeat1 = config.SetpointUnoccupiedHeat1
	}

	if config.SetpointStandbyCool1 != nil {
		setup.SetpointStandbyCool1 = config.SetpointStandbyCool1
	}

	if config.SetpointStandbyHeat1 != nil {
		setup.SetpointStandbyHeat1 = config.SetpointStandbyHeat1
	}

	if config.HvacsTargetMode != nil {
		setup.HvacsTargetMode = config.HvacsTargetMode
	}

	if config.HvacsHeatCool != nil {
		setup.HvacsHeatCool = config.HvacsHeatCool
	}

	return db.UpdateRecord(pconst.DbConfig, pconst.TbGroups, dbID, setup)
}

// func UpdateGroupConfig(db Database, cfg gm.GroupConfig) error {
// 	criteria := make(map[string]interface{})
// 	criteria["Group"] = cfg.Group
// 	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, pconst.TbGroups, criteria)
// }

func GetGroupsConfig(db Database) map[int]gm.GroupConfig {
	groups := make(map[int]gm.GroupConfig)
	stored, err := db.FetchAllRecords(pconst.DbConfig, pconst.TbGroups)
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
