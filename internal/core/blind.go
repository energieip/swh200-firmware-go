package core

import (
	"encoding/json"
	"time"

	"github.com/energieip/common-components-go/pkg/dblind"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

func (s *Service) sendBlindSetup(driver dblind.BlindSetup) {
	url := "/write/blind/" + driver.Mac + "/" + dblind.UrlSetup
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendBlindUpdate(driver dblind.BlindConf) {
	url := "/write/blind/" + driver.Mac + "/" + dblind.UrlSetting
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendBlindGroupSetpoint(mac string, blind *int, slat *int) {
	_, ok := s.blinds[mac]
	if !ok {
		rlog.Warn("Blind " + mac + " not plugged to this switch")
		return
	}
	conf := dblind.BlindConf{
		Mac: mac,
	}
	if blind != nil {
		conf.Blind1 = blind
		conf.Blind2 = blind
	}
	if slat != nil {
		conf.Slat1 = slat
		conf.Slat2 = slat
	}
	s.sendBlindUpdate(conf)
}

func (s *Service) removeBlind(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(dblind.DbConfig, dblind.TableName, criteria)
	_, ok := s.blinds[mac]
	if !ok {
		return
	}
	isConfigured := false
	remove := dblind.BlindConf{
		Mac:          mac,
		IsConfigured: &isConfigured,
	}
	s.sendBlindUpdate(remove)
}

func (s *Service) updateBlindStatus(driver dblind.Blind) error {
	var err error
	val, ok := s.blinds[driver.Mac]
	if ok && val == driver {
		//case no change
		return nil
	}

	// Check if the serial already exist in database (case restart process)
	criteria := make(map[string]interface{})
	criteria["Mac"] = driver.Mac
	dbID := database.GetObjectID(s.db, dblind.DbStatus, dblind.TableName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(dblind.DbStatus, dblind.TableName, driver)
	} else {
		err = s.db.UpdateRecord(dblind.DbStatus, dblind.TableName, dbID, driver)
	}
	if err == nil {
		s.blinds[driver.Mac] = driver
	}
	return err
}

func (s *Service) prepareBlindSetup(driver dblind.BlindSetup) {
	var err error
	criteria := make(map[string]interface{})
	criteria["Mac"] = driver.Mac
	dbID := database.GetObjectID(s.db, dblind.DbConfig, dblind.TableName, criteria)

	if dbID == "" {
		_, err = s.db.InsertRecord(dblind.DbConfig, dblind.TableName, driver)
	} else {
		err = s.db.UpdateRecord(dblind.DbConfig, dblind.TableName, dbID, driver)
	}
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	bld, ok := s.blinds[driver.Mac]
	if ok && !bld.IsConfigured {
		s.sendBlindSetup(driver)
	}
}

func (s *Service) updateBlindConfig(cfg dblind.BlindConf) {
	setup, dbID := s.getBlindConfig(cfg.Mac)
	if setup == nil || dbID == "" {
		return
	}

	if cfg.FriendlyName != nil {
		setup.FriendlyName = cfg.FriendlyName
	}

	if cfg.Group != nil {
		setup.Group = cfg.Group
	}

	if cfg.IsBleEnabled != nil {
		setup.IsBleEnabled = cfg.IsBleEnabled
	}

	if cfg.DumpFrequency != nil {
		setup.DumpFrequency = *cfg.DumpFrequency
	}

	err := s.db.UpdateRecord(dblind.DbConfig, dblind.TableName, dbID, setup)
	if err != nil {
		rlog.Error("Cannot update database" + err.Error())
		return
	}
	_, ok := s.blinds[cfg.Mac]
	if ok {
		s.sendBlindUpdate(cfg)
	}
}

func (s *Service) onBlindHello(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var driver dblind.Blind
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen[driver.Mac] = time.Now().UTC()
	if driver.DumpFrequency == 0 {
		driver.DumpFrequency = 1000 //ms default value for hello
	}

	driver.IsConfigured = false
	driver.SwitchMac = s.mac
	err = s.updateBlindStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	rlog.Debug("New Blind driver stored on database " + driver.Mac)

	cfg := database.GetConfigBlind(s.db, driver.Mac)
	if cfg != nil {
		s.sendBlindSetup(*cfg)
	}
}

func (s *Service) onBlindStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug(topic + " : " + string(msg.Payload()))
	var driver dblind.Blind
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen[driver.Mac] = time.Now().UTC()
	driver.SwitchMac = s.mac
	err = s.updateBlindStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
}

func (s *Service) getBlindConfig(mac string) (*dblind.BlindSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(dblind.DbConfig, dblind.TableName, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dblind.ToBlindSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}
