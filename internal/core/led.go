package core

import (
	"encoding/json"
	"time"

	dl "github.com/energieip/common-components-go/pkg/dled"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/common-components-go/pkg/pconst"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

func (s *Service) sendLedSetup(led dl.LedSetup) {
	url := "/write/led/" + led.Mac + "/" + pconst.UrlSetup
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
	url := "/write/led/" + led.Mac + "/" + pconst.UrlSetting
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
	s.db.DeleteRecord(pconst.DbConfig, pconst.TbLeds, criteria)
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
	s.leds.Remove(mac)
	s.sendLedUpdate(remove)
	s.driversSeen.Remove(mac)
}

func (s *Service) updateLedStatus(led dl.Led) error {
	var err error
	v, ok := s.leds.Get(led.Mac)
	if ok && v != nil {
		val := v.(dl.Led)
		if val == led {
			//case no change
			return nil
		}
	}

	// Check if the serial already exist in database (case restart process)
	err = database.SaveLedStatus(s.db, led)
	if err == nil {
		s.leds.Set(led.Mac, led)
	}
	return err
}

func (s *Service) prepareLedSetup(led dl.LedSetup) {
	err := database.SaveLedSetup(s.db, led)
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
	setup, dbID := database.GetLedConfig(s.db, config.Mac)
	if setup == nil || dbID == "" {
		return
	}

	new := dl.UpdateConfig(config, *setup)
	err := s.db.UpdateRecord(pconst.DbConfig, pconst.TbLeds, dbID, &new)
	if err != nil {
		rlog.Error("Error updating database " + err.Error())
		return
	}
	l, ok := s.leds.Get(config.Mac)
	if ok {
		light := l.(dl.Led)
		if light.IsConfigured {
			s.sendLedUpdate(config)
		}
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
	if cfg != nil {
		if cfg.Watchdog != nil {
			led.Watchdog = *cfg.Watchdog
		}
		if cfg.FirstDay != nil {
			led.FirstDay = *cfg.FirstDay
		}
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
