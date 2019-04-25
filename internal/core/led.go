package core

import (
	"encoding/json"
	"time"

	dl "github.com/energieip/common-components-go/pkg/dled"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

func (s *Service) sendLedSetup(led dl.LedSetup) {
	url := "/write/led/" + led.Mac + "/" + dl.UrlSetup
	dump, _ := led.ToJSON()
	s.localSendCommand(url, dump)
	if led.Auto != nil {
		if *led.Auto == false {
			//start countdown
			watchdog := 0
			if led.Watchdog != nil {
				watchdog = *led.Watchdog
			} else {
				cfg := database.GetConfigLed(s.db, led.Mac)
				if cfg.Watchdog != nil {
					watchdog = *cfg.Watchdog
				}
			}
			s.ledsToAuto[led.Mac] = &watchdog
		} else {
			_, ok := s.ledsToAuto[led.Mac]
			if ok {
				delete(s.ledsToAuto, led.Mac)
			}
		}
	}
}

func (s *Service) sendLedUpdate(led dl.LedConf) {
	url := "/write/led/" + led.Mac + "/" + dl.UrlSetting
	dump, _ := led.ToJSON()
	s.localSendCommand(url, dump)

	if led.Auto != nil {
		if *led.Auto == false {
			//start countdown
			watchdog := 0
			if led.Watchdog != nil {
				watchdog = *led.Watchdog
			} else {
				cfg := database.GetConfigLed(s.db, led.Mac)
				if cfg.Watchdog != nil {
					watchdog = *cfg.Watchdog
				}
			}
			s.ledsToAuto[led.Mac] = &watchdog
		} else {
			_, ok := s.ledsToAuto[led.Mac]
			if ok {
				delete(s.ledsToAuto, led.Mac)
			}
		}
	}
}

func (s *Service) sendLedGroupSetpoint(mac string, setpoint int, slopeStart int, slopeStop int) {
	_, ok := s.leds.Get(mac)
	if !ok {
		return
	}
	conf := dl.LedConf{
		Mac:            mac,
		SetpointAuto:   &setpoint,
		SlopeStartAuto: &slopeStart,
		SlopeStopAuto:  &slopeStop,
	}
	s.sendLedUpdate(conf)
}

func (s *Service) removeLed(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(dl.DbConfig, dl.TableName, criteria)
	_, ok := s.ledsToAuto[mac]
	if ok {
		delete(s.ledsToAuto, mac)
	}
	_, ok = s.leds.Get(mac)
	if !ok {
		return
	}
	isConfigured := false
	remove := dl.LedConf{
		Mac:          mac,
		IsConfigured: &isConfigured,
	}
	s.sendLedUpdate(remove)
	s.driversSeen.Remove(mac)
}

func (s *Service) updateLedStatus(led dl.Led) error {
	var err error
	v, ok := s.leds.Get(led.Mac)
	if ok {
		val := v.(dl.Led)
		if val == led {
			//case no change
			return nil
		}
	}

	// Check if the serial already exist in database (case restart process)
	criteria := make(map[string]interface{})
	criteria["Mac"] = led.Mac
	dbID := database.GetObjectID(s.db, dl.DbStatus, dl.TableName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(dl.DbStatus, dl.TableName, led)
	} else {
		err = s.db.UpdateRecord(dl.DbStatus, dl.TableName, dbID, led)
	}
	if err == nil {
		s.leds.Set(led.Mac, led)
	}
	return err
}

func (s *Service) prepareLedSetup(led dl.LedSetup) {
	var err error
	criteria := make(map[string]interface{})
	criteria["Mac"] = led.Mac
	dbID := database.GetObjectID(s.db, dl.DbConfig, dl.TableName, criteria)

	if dbID == "" {
		_, err = s.db.InsertRecord(dl.DbConfig, dl.TableName, led)
	} else {
		err = s.db.UpdateRecord(dl.DbConfig, dl.TableName, dbID, led)
	}
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	l, ok := s.leds.Get(led.Mac)
	if ok {
		light := l.(dl.Led)
		if !light.IsConfigured {
			s.sendLedSetup(led)
		}
	}
}

func (s *Service) updateLedConfig(config dl.LedConf) {
	setup, dbID := s.getLedConfig(config.Mac)
	if setup == nil || dbID == "" {
		return
	}

	if config.ThresholdHigh != nil {
		setup.ThresholdHigh = config.ThresholdHigh
	}

	if config.ThresholdLow != nil {
		setup.ThresholdLow = config.ThresholdLow
	}

	if config.FriendlyName != nil {
		setup.FriendlyName = config.FriendlyName
	}

	if config.Group != nil {
		setup.Group = config.Group
	}

	if config.IsBleEnabled != nil {
		setup.IsBleEnabled = config.IsBleEnabled
	}

	if config.SlopeStartAuto != nil {
		setup.SlopeStartAuto = config.SlopeStartAuto
	}

	if config.SlopeStartManual != nil {
		setup.SlopeStartManual = config.SlopeStartManual
	}

	if config.SlopeStopAuto != nil {
		setup.SlopeStopAuto = config.SlopeStopAuto
	}

	if config.SlopeStopManual != nil {
		setup.SlopeStopManual = config.SlopeStopManual
	}

	if config.BleMode != nil {
		setup.BleMode = config.BleMode
	}

	if config.IBeaconMajor != nil {
		setup.IBeaconMajor = config.IBeaconMajor
	}

	if config.IBeaconMinor != nil {
		setup.IBeaconMinor = config.IBeaconMinor
	}

	if config.IBeaconTxPower != nil {
		setup.IBeaconTxPower = config.IBeaconTxPower
	}

	if config.IBeaconUUID != nil {
		setup.IBeaconUUID = config.IBeaconUUID
	}

	if config.DumpFrequency != nil {
		setup.DumpFrequency = *config.DumpFrequency
	}

	if config.Watchdog != nil {
		setup.Watchdog = config.Watchdog
	}

	err := s.db.UpdateRecord(dl.DbConfig, dl.TableName, dbID, setup)
	if err != nil {
		rlog.Error("Error updating database " + err.Error())
		return
	}
	_, ok := s.leds.Get(config.Mac)
	if ok {
		s.sendLedUpdate(config)
	}
}

func (s *Service) onLedHello(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var led dl.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen.Set(led.Mac, time.Now().UTC())
	led.IsConfigured = false
	led.SwitchMac = s.mac
	if led.DumpFrequency == 0 {
		led.DumpFrequency = 1000 //ms default value for hello
	}
	err = s.updateLedStatus(led)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	rlog.Debugf("New LED driver %v stored on database ", led.Mac)

	cfg := database.GetConfigLed(s.db, led.Mac)
	if cfg != nil {
		s.sendLedSetup(*cfg)
	}
}

func (s *Service) onLedStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug(topic + " : " + string(msg.Payload()))
	var led dl.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen.Set(led.Mac, time.Now().UTC())
	led.SwitchMac = s.mac
	val, ok := s.ledsToAuto[led.Mac]
	if ok && val != nil {
		led.TimeToAuto = *val
	}
	cfg := database.GetConfigLed(s.db, led.Mac)
	if cfg != nil && cfg.Watchdog != nil {
		led.Watchdog = *cfg.Watchdog
	}

	err = s.updateLedStatus(led)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
}

func (s *Service) cronLedMode() {
	timerDump := time.NewTicker(time.Second)
	for {
		select {
		case <-timerDump.C:
			for mac, val := range s.ledsToAuto {
				if val == nil {
					continue
				}

				if *val <= 0 {
					auto := true

					cfg := dl.LedConf{
						Mac:  mac,
						Auto: &auto,
					}
					s.sendLedUpdate(cfg)
					delete(s.ledsToAuto, mac)
				} else {
					tempo := *val
					tempo--
					s.ledsToAuto[mac] = &tempo
				}
			}
		}
	}
}

func (s *Service) getLedConfig(mac string) (*dl.LedSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(dl.DbConfig, dl.TableName, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := dl.ToLedSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}
