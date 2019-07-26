package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/pconst"

	ds "github.com/energieip/common-components-go/pkg/dsensor"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

type SensorEvent struct {
	Mac         string `json:"mac"`
	Temperature int    `json:"temperature"`
	Brightness  int    `json:"brightness"`
	Humidity    int    `json:"humidity"`
	Presence    bool   `json:"presence"`
}

type SensorErrorEvent struct {
	Mac string `json:"mac"`
}

//ToJSON dump struct in json
func (sensor SensorErrorEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(sensor)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToSensorErrorEvent convert interface to Sensor object
func ToSensorErrorEvent(val interface{}) (*SensorErrorEvent, error) {
	var cell SensorErrorEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &cell)
	return &cell, err
}

//ToJSON dump struct in json
func (sensor SensorEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(sensor)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToSensorEvent convert interface to Sensor object
func ToSensorEvent(val interface{}) (*SensorEvent, error) {
	var cell SensorEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &cell)
	return &cell, err
}

func (s *Service) sendSensorSetup(sensor ds.SensorSetup) {
	url := "/write/sensor/" + sensor.Mac + "/" + pconst.UrlSetup
	dump, _ := sensor.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendSensorUpdate(sensor ds.SensorConf) {
	url := "/write/sensor/" + sensor.Mac + "/" + pconst.UrlSetting
	dump, _ := sensor.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) updateSensorStatus(sensor ds.Sensor) error {
	v, ok := s.sensors.Get(sensor.Mac)
	if ok && v != nil {
		val := v.(ds.Sensor)
		if val == sensor {
			return nil
		}
	}

	err := database.SaveSensorStatus(s.db, sensor)
	if err == nil {
		s.sensors.Set(sensor.Mac, sensor)
	}
	return err
}

func (s *Service) prepareSensorSetup(sensor ds.SensorSetup) {
	err := database.SaveSensorSetup(s.db, sensor)
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	c, ok := s.sensors.Get(sensor.Mac)
	if ok {
		cell := c.(ds.Sensor)
		if !cell.IsConfigured {
			//uncomment this is due to an issue on sensor side
			// s.sendSensorSetup(sensor)
		}
	}
}

func (s *Service) updateSensorConfig(cfg ds.SensorConf) {
	setup, dbID := database.GetSensorConfig(s.db, cfg.Mac)
	if setup == nil || dbID == "" {
		return
	}
	new := ds.UpdateConfig(cfg, *setup)
	err := s.db.UpdateRecord(pconst.DbConfig, pconst.TbSensors, dbID, &new)
	if err != nil {
		rlog.Error("Error updating database" + err.Error())
		return
	}
	_, ok := s.sensors.Get(cfg.Mac)
	if ok {
		s.sendSensorUpdate(cfg)
	}
}

func (s *Service) removeSensor(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(pconst.DbConfig, pconst.TbSensors, criteria)
	_, ok := s.sensors.Get(mac)
	if ok {
		isConfigured := false
		remove := ds.SensorConf{
			Mac:          mac,
			IsConfigured: &isConfigured,
		}
		s.sensors.Remove(mac)
		s.sendSensorUpdate(remove)
		s.driversSeen.Remove(mac)
	}
}

func (s *Service) onSensorHello(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var sensor ds.Sensor
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	sensor.Mac = strings.ToUpper(sensor.Mac)
	s.driversSeen.Set(sensor.Mac, time.Now().UTC())
	sensor.IsConfigured = false
	sensor.SwitchMac = s.mac
	if sensor.DumpFrequency == 0 {
		sensor.DumpFrequency = 1000 //ms default value for hello
	}
	err = s.updateSensorStatus(sensor)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	cfg := database.GetConfigSensor(s.db, sensor.Mac)
	if cfg != nil {
		s.sendSensorSetup(*cfg)
	}
}

func (s *Service) onSensorStatus(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var sensor ds.Sensor
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	sensor.Mac = strings.ToUpper(sensor.Mac)
	s.driversSeen.Set(sensor.Mac, time.Now().UTC())
	sensor.SwitchMac = s.mac
	// apply brightness correction Factor
	sensor.Brightness = sensor.BrightnessRaw / sensor.BrightnessCorrectionFactor

	err = s.updateSensorStatus(sensor)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}

	if sensor.Error == 0 {
		url := "/read/group/" + strconv.Itoa(sensor.Group) + "/events/sensor"
		evt := SensorEvent{
			Mac:         sensor.Mac,
			Temperature: sensor.Temperature,
			Humidity:    sensor.Humidity,
			Brightness:  sensor.Brightness,
			Presence:    sensor.Presence,
		}
		dump, _ := evt.ToJSON()
		s.clusterSendCommand(url, dump)
		s.localSendCommand(url, dump)
	} else {
		s.sendInvalidStatus(sensor)
	}
}

func (s *Service) sendInvalidStatus(sensor ds.Sensor) {
	url := "/read/group/" + strconv.Itoa(sensor.Group) + "/error/sensor"
	evt := SensorErrorEvent{
		Mac: sensor.Mac,
	}
	dump, _ := evt.ToJSON()

	s.clusterSendCommand(url, dump)
	s.localSendCommand(url, dump)
}
