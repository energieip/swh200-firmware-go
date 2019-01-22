package core

import (
	"encoding/json"
	"time"

	dl "github.com/energieip/common-components-go/pkg/dled"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

func (s *Service) sendLedSetup(led dl.LedSetup) {
	url := "/write/led/" + led.Mac + "/" + dl.UrlSetup
	dump, _ := led.ToJSON()

	err := s.localSendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration for driver " + led.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + led.Mac + " on topic: " + url + " dump: " + dump)
	}
}

func (s *Service) sendLedUpdate(led dl.LedConf) {
	url := "/write/led/" + led.Mac + "/" + dl.UrlSetting
	dump, _ := led.ToJSON()

	err := s.localSendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new settings for driver " + led.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New settings has been sent to " + led.Mac + " on topic: " + url + " dump: " + dump)
	}
}

func (s *Service) sendLedGroupSetpoint(mac string, setpoint int) {
	_, ok := s.leds[mac]
	if !ok {
		rlog.Warn("Led " + mac + " not plugged to this switch")
		return
	}
	conf := dl.LedConf{
		Mac:          mac,
		SetpointAuto: &setpoint,
	}
	s.sendLedUpdate(conf)
}

func (s *Service) removeLed(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(dl.DbConfig, dl.TableName, criteria)
	_, ok := s.leds[mac]
	if !ok {
		return
	}
	isConfigured := false
	remove := dl.LedConf{
		Mac:          mac,
		IsConfigured: &isConfigured,
	}
	s.sendLedUpdate(remove)
}

func (s *Service) updateLedStatus(led dl.Led) error {
	var err error
	val, ok := s.leds[led.Mac]
	if ok && val == led {
		//case no change
		return nil
	}

	// Check if the serial already exist in database (case restart process)
	criteria := make(map[string]interface{})
	criteria["Mac"] = led.Mac
	dbID := s.getObjectID(dl.DbStatus, dl.TableName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(dl.DbStatus, dl.TableName, led)
	} else {
		err = s.db.UpdateRecord(dl.DbStatus, dl.TableName, dbID, led)
	}
	if err == nil {
		s.leds[led.Mac] = led
	}
	return err
}

func (s *Service) prepareLedSetup(led dl.LedSetup) {
	var err error
	criteria := make(map[string]interface{})
	criteria["Mac"] = led.Mac
	dbID := s.getObjectID(dl.DbConfig, dl.TableName, criteria)

	if dbID == "" {
		_, err = s.db.InsertRecord(dl.DbConfig, dl.TableName, led)
	} else {
		err = s.db.UpdateRecord(dl.DbConfig, dl.TableName, dbID, led)
	}
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	_, ok := s.leds[led.Mac]
	if ok {
		s.sendLedSetup(led)
	}
}

func (s *Service) updateLedConfig(led dl.LedConf) {
	_, ok := s.leds[led.Mac]
	if ok {
		s.sendLedUpdate(led)
	}
}

func (s *Service) onLedHello(client network.Client, msg network.Message) {
	rlog.Info("LED service: Received hello topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var led dl.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen[led.Mac] = time.Now().UTC()
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

	cfg := s.getConfigLed(led.Mac)
	if cfg != nil {
		s.sendLedSetup(*cfg)
	}
}

func (s *Service) onLedStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug("LED service driver status: Received topic: " + topic + " payload: " + string(msg.Payload()))
	var led dl.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen[led.Mac] = time.Now().UTC()
	led.SwitchMac = s.mac
	err = s.updateLedStatus(led)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
}
