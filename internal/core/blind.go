package core

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/energieip/common-components-go/pkg/dblind"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

type BlindEvent struct {
	Mac           string `json:"mac"`
	WindowStatus1 bool   `json:"windowStatus1"`
	WindowStatus2 bool   `json:"windowStatus2"`
}

type BlindErrorEvent struct {
	Mac string `json:"mac"`
}

//ToJSON dump struct in json
func (driver BlindErrorEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
}

//ToBlindErrorEvent convert interface to Blind object
func ToBlindErrorEvent(val interface{}) (*BlindErrorEvent, error) {
	var driver BlindErrorEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

//ToJSON dump struct in json
func (driver BlindEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
}

//ToBlindEvent convert interface to Blind object
func ToBlindEvent(val interface{}) (*BlindEvent, error) {
	var driver BlindEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

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
	_, ok := s.blinds.Get(mac)
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
	_, ok := s.blinds.Get(mac)
	if !ok {
		return
	}
	isConfigured := false
	remove := dblind.BlindConf{
		Mac:          mac,
		IsConfigured: &isConfigured,
	}
	s.sendBlindUpdate(remove)
	s.blinds.Remove(mac)
	s.driversSeen.Remove(mac)
}

func (s *Service) updateBlindStatus(driver dblind.Blind) error {
	var err error
	// v, ok := s.blinds.Get(driver.Mac)
	// if ok && v != nil {
	// 	val := v.(dblind.Blind)
	// 	if val == driver {
	// 		//case no change
	// 		return nil
	// 	}
	// }

	// Check if the serial already exist in database (case restart process)
	err = database.SaveBlindStatus(s.db, driver)
	if err == nil {
		s.blinds.Set(driver.Mac, driver)
	}
	return err
}

func (s *Service) prepareBlindSetup(driver dblind.BlindSetup) {
	err := database.SaveBlindSetup(s.db, driver)
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	bld, ok := s.blinds.Get(driver.Mac)
	if ok {
		blind := bld.(dblind.Blind)
		if !blind.IsConfigured {
			s.sendBlindSetup(driver)
		}
	}
}

func (s *Service) updateBlindConfig(cfg dblind.BlindConf) {
	setup, dbID := database.GetBlindConfig(s.db, cfg.Mac)
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

	if cfg.BleMode != nil {
		setup.BleMode = cfg.BleMode
	}

	if cfg.IBeaconMajor != nil {
		setup.IBeaconMajor = cfg.IBeaconMajor
	}

	if cfg.IBeaconMinor != nil {
		setup.IBeaconMinor = cfg.IBeaconMinor
	}

	if cfg.IBeaconTxPower != nil {
		setup.IBeaconTxPower = cfg.IBeaconTxPower
	}

	if cfg.IBeaconUUID != nil {
		setup.IBeaconUUID = cfg.IBeaconUUID
	}

	if cfg.DumpFrequency != nil {
		setup.DumpFrequency = *cfg.DumpFrequency
	}

	err := s.db.UpdateRecord(dblind.DbConfig, dblind.TableName, dbID, setup)
	if err != nil {
		rlog.Error("Cannot update database" + err.Error())
		return
	}
	_, ok := s.blinds.Get(cfg.Mac)
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
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
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

func (s *Service) sendInvalidBlindStatus(driver dblind.Blind) {
	url := "/read/group/" + strconv.Itoa(driver.Group) + "/error/blind"
	evt := BlindErrorEvent{
		Mac: driver.Mac,
	}
	dump, _ := evt.ToJSON()

	s.clusterSendCommand(url, dump)
	s.localSendCommand(url, dump)
}

func (s *Service) onBlindStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Info(topic + " : " + string(msg.Payload()))
	var driver dblind.Blind
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	driver.SwitchMac = s.mac
	err = s.updateBlindStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
	if driver.Error == 0 {
		url := "/read/group/" + strconv.Itoa(driver.Group) + "/events/blind"
		evt := BlindEvent{
			Mac:           driver.Mac,
			WindowStatus1: driver.WindowStatus1,
			WindowStatus2: driver.WindowStatus2,
		}
		dump, _ := evt.ToJSON()
		s.clusterSendCommand(url, dump)
		s.localSendCommand(url, dump)
	} else {
		s.sendInvalidBlindStatus(driver)
	}
}
