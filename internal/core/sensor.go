package core

import (
	"encoding/json"
	"strconv"
	"time"

	ds "github.com/energieip/common-components-go/pkg/dsensor"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

type SensorEvent struct {
	Mac         string `json:"mac"`
	Temperature int    `json:"temperature"`
	Brightness  int    `json:"brightness"`
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
	return string(inrec[:]), err
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
	return string(inrec[:]), err
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
	url := "/write/sensor/" + sensor.Mac + "/" + ds.UrlSetup
	dump, _ := sensor.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) sendSensorUpdate(sensor ds.SensorConf) {
	url := "/write/sensor/" + sensor.Mac + "/" + ds.UrlSetting
	dump, _ := sensor.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) updateSensorStatus(sensor ds.Sensor) error {
	val, ok := s.sensors[sensor.Mac]
	if ok && val == sensor {
		return nil
	}

	var err error
	criteria := make(map[string]interface{})
	criteria["Mac"] = sensor.Mac
	dbID := database.GetObjectID(s.db, ds.DbStatus, ds.TableName, criteria)
	if dbID == "" {
		_, err = s.db.InsertRecord(ds.DbStatus, ds.TableName, sensor)
	} else {
		err = s.db.UpdateRecord(ds.DbStatus, ds.TableName, dbID, sensor)
	}
	if err == nil {
		s.sensors[sensor.Mac] = sensor
	}
	return err
}

func (s *Service) prepareSensorSetup(sensor ds.SensorSetup) {
	var err error
	criteria := make(map[string]interface{})
	criteria["Mac"] = sensor.Mac
	dbID := database.GetObjectID(s.db, ds.DbConfig, ds.TableName, criteria)

	if dbID == "" {
		_, err = s.db.InsertRecord(ds.DbConfig, ds.TableName, sensor)
	} else {
		err = s.db.UpdateRecord(ds.DbConfig, ds.TableName, dbID, sensor)
	}
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	cell, ok := s.sensors[sensor.Mac]
	if ok && !cell.IsConfigured {
		//uncomment this is due to an issue on sensor side
		// s.sendSensorSetup(sensor)
	}
}

func (s *Service) updateSensorConfig(cfg ds.SensorConf) {
	setup, dbID := s.getSensorConfig(cfg.Mac)
	if setup == nil || dbID == "" {
		return
	}

	if cfg.BrightnessCorrectionFactor != nil {
		setup.BrightnessCorrectionFactor = cfg.BrightnessCorrectionFactor
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

	if cfg.TemperatureOffset != nil {
		setup.TemperatureOffset = cfg.TemperatureOffset
	}

	if cfg.ThresholdPresence != nil {
		setup.ThresholdPresence = cfg.ThresholdPresence
	}

	if cfg.DumpFrequency != nil {
		setup.DumpFrequency = *cfg.DumpFrequency
	}

	err := s.db.UpdateRecord(ds.DbConfig, ds.TableName, dbID, setup)
	if err != nil {
		rlog.Error("Error updating database" + err.Error())
		return
	}
	_, ok := s.sensors[cfg.Mac]
	if ok {
		s.sendSensorUpdate(cfg)
	}
}

func (s *Service) removeSensor(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(ds.DbConfig, ds.TableName, criteria)
	_, ok := s.sensors[mac]
	if ok {
		isConfigured := false
		remove := ds.SensorConf{
			Mac:          mac,
			IsConfigured: &isConfigured,
		}
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

func (s *Service) getSensorConfig(mac string) (*ds.SensorSetup, string) {
	var dbID string
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	stored, err := s.db.GetRecord(ds.DbConfig, ds.TableName, criteria)
	if err != nil || stored == nil {
		return nil, dbID
	}
	m := stored.(map[string]interface{})
	id, ok := m["id"]
	if ok {
		dbID = id.(string)
	}
	driver, err := ds.ToSensorSetup(stored)
	if err != nil {
		return nil, dbID
	}
	return driver, dbID
}
