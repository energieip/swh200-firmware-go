package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	dn "github.com/energieip/common-components-go/pkg/dnanosense"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

type NanoEvent struct {
	Mac         string `json:"mac"`
	Hygrometry  int    `json:"hygrometry"`
	Temperature int    `json:"temperature"`
	CO2         int    `json:"co2"`
	COV         int    `json:"cov"`
}

type NanoErrorEvent struct {
	Mac string `json:"mac"`
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
	s.nanos.Set(driver.Mac, driver)
	return nil
}

func (s *Service) sendInvalidNanoStatus(driver dn.Nanosense) {
	url := "/read/group/" + strconv.Itoa(driver.Group) + "/error/nano"
	evt := NanoErrorEvent{
		Mac: driver.Mac,
	}
	dump, _ := evt.ToJSON()
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
	driver.Mac = strings.ToUpper(driver.Mac)
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	err = s.updateNanoStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
	if driver.Error == 0 {
		url := "/read/group/" + strconv.Itoa(driver.Group) + "/events/nano"
		evt := NanoEvent{
			Mac:         driver.Mac,
			Hygrometry:  driver.Hygrometry,
			Temperature: driver.Temperature,
			CO2:         driver.CO2,
			COV:         driver.COV,
		}
		dump, _ := evt.ToJSON()
		s.localSendCommand(url, dump)
	} else {
		s.sendInvalidNanoStatus(driver)
	}
}
