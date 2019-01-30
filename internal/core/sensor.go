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

func (s *Service) updateSensorConfig(sensor ds.SensorConf) {
	_, ok := s.sensors[sensor.Mac]
	if ok {
		s.sendSensorUpdate(sensor)
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
	}
}

func (s *Service) onSensorHello(client network.Client, msg network.Message) {
	rlog.Debug(msg.Topic() + " : " + string(msg.Payload()))
	var sensor ds.Sensor
	err := json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen[sensor.Mac] = time.Now().UTC()
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
	rlog.Debug("New Sensor driver stored on database :" + sensor.Mac)
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
	s.driversSeen[sensor.Mac] = time.Now().UTC()
	sensor.SwitchMac = s.mac
	// apply brightness correction Factor
	sensor.Brightness = sensor.BrightnessRaw / sensor.BrightnessCorrectionFactor

	err = s.updateSensorStatus(sensor)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}

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
}
