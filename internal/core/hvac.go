package core

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/dhvac"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/common-components-go/pkg/pconst"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

func (s *Service) sendHvacSetup(driver dhvac.HvacSetup) {
	url := "/write/hvac/" + driver.Mac + "/" + pconst.UrlSetup
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendHvacUpdate(driver dhvac.HvacConf) {
	url := "/write/hvac/" + driver.Mac + "/" + pconst.UrlSetting
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendHvacGroupSetpoint(mac string, temperatureOffset *int) {
	_, ok := s.hvacs.Get(mac)
	if !ok {
		rlog.Warn("HVAC " + mac + " not plugged to this switch")
		return
	}
	conf := dhvac.HvacConf{
		Mac:   mac,
		Shift: temperatureOffset,
	}
	s.sendHvacUpdate(conf)
}

func (s *Service) sendHvacSpaceValues(mac string, temperature int, co2 int, cov int, hygrometry int, opened bool, presence bool) {
	_, ok := s.hvacs.Get(mac)
	if !ok {
		rlog.Warn("HVAC " + mac + " not plugged to this switch")
		return
	}
	conf := dhvac.HvacConf{
		Mac:          mac,
		WindowStatus: &opened,
		Temperature:  &temperature,
		CO2:          &co2,
		COV:          &cov,
		Hygrometry:   &hygrometry,
		Presence:     &presence,
	}
	s.sendHvacUpdate(conf)
}

func (s *Service) removeHvac(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(pconst.DbConfig, pconst.TbHvacs, criteria)
	_, ok := s.hvacs.Get(mac)
	if !ok {
		return
	}
	isConfigured := false
	remove := dhvac.HvacConf{
		Mac: mac,
	}
	remove.IsConfigured = &isConfigured
	s.sendHvacUpdate(remove)
	s.driversSeen.Remove(mac)
}

func (s *Service) updateHvacStatus(driver dhvac.Hvac) error {
	s.hvacs.Set(driver.Mac, driver)
	return nil
}

func (s *Service) prepareHvacSetup(driver dhvac.HvacSetup) {
	err := database.SaveHvacSetup(s.db, driver)
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	drv, ok := s.hvacs.Get(driver.Mac)
	if ok {
		driv := drv.(dhvac.Hvac)
		if !driv.IsConfigured {
			s.sendHvacSetup(driver)
		}
	}
}

func (s *Service) updateHvacConfig(cfg dhvac.HvacConf) {
	setup, dbID := database.GetHvacConfig(s.db, cfg.Mac)
	if setup == nil || dbID == "" {
		return
	}
	new := dhvac.UpdateConfig(cfg, *setup)
	err := s.db.UpdateRecord(pconst.DbConfig, pconst.TbHvacs, dbID, &new)
	if err != nil {
		rlog.Error("Cannot update database" + err.Error())
		return
	}
	_, ok := s.hvacs.Get(cfg.Mac)
	if ok {
		s.sendHvacUpdate(cfg)
	}
}

func (s *Service) onHvacHello(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var driver dhvac.Hvac
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	driver.Mac = strings.ToUpper(driver.Mac)
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	if driver.DumpFrequency == 0 {
		driver.DumpFrequency = 1000 //ms default value for hello
	}

	driver.IsConfigured = false
	driver.SwitchMac = s.mac

	err = s.updateHvacStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	rlog.Debug("New Hvac driver stored on database " + driver.Mac)

	cfg := database.GetConfigHvac(s.db, driver.Mac)
	if cfg != nil {
		s.sendHvacSetup(*cfg)
	}
}

func (s *Service) onHvacStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug(topic + " : " + string(msg.Payload()))
	var driver dhvac.Hvac
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	driver.Mac = strings.ToUpper(driver.Mac)
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	driver.SwitchMac = s.mac

	s.updateHvacStatus(driver)
}
