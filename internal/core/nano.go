package core

import (
	"encoding/json"
	"strconv"
	"time"

	dn "github.com/energieip/common-components-go/pkg/dnanosense"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

type NanoEvent struct {
	Label       string `json:"label"`
	Hygrometry  int    `json:"hygrometry"`
	Temperature int    `json:"temperature"`
	CO2         int    `json:"co2"`
	COV         int    `json:"cov"`
}

type NanoErrorEvent struct {
	Label string `json:"label"`
}

//ToJSON dump struct in json
func (driver NanoErrorEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToNanoErrorEvent convert interface to Blind object
func ToNanoErrorEvent(val interface{}) (*NanoErrorEvent, error) {
	var driver NanoErrorEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

//ToJSON dump struct in json
func (driver NanoEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToNanoEvent convert interface to nano object
func ToNanoEvent(val interface{}) (*NanoEvent, error) {
	var driver NanoEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

func (s *Service) updateNanoStatus(driver dn.Nanosense) error {
	var err error
	v, ok := s.nanos.Get(driver.Label)
	if ok && v != nil {
		val := v.(dn.Nanosense)
		if val == driver {
			//case no change
			return nil
		}
	}

	// Check if the serial already exist in database (case restart process)
	err = database.SaveNanoStatus(s.db, driver)
	if err == nil {
		s.nanos.Set(driver.Label, driver)
	}
	return err
}

func (s *Service) sendInvalidNanoStatus(driver dn.Nanosense) {
	url := "/read/group/" + strconv.Itoa(driver.Group) + "/error/nano"
	evt := NanoErrorEvent{
		Label: driver.Label,
	}
	dump, _ := evt.ToJSON()

	s.clusterSendCommand(url, dump)
	s.localSendCommand(url, dump)
}

func (s *Service) onNanoStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Info(topic + " : " + string(msg.Payload()))
	var driver dn.Nanosense
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	s.driversSeen.Set(driver.Label, time.Now().UTC())
	err = s.updateNanoStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
	if driver.Error == 0 {
		url := "/read/group/" + strconv.Itoa(driver.Group) + "/events/nano"
		evt := NanoEvent{
			Label:       driver.Label,
			Hygrometry:  driver.Hygrometry,
			Temperature: driver.Temperature,
			CO2:         driver.CO2,
			COV:         driver.COV,
		}
		dump, _ := evt.ToJSON()
		s.clusterSendCommand(url, dump)
		s.localSendCommand(url, dump)
	} else {
		s.sendInvalidNanoStatus(driver)
	}
}
