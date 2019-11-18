package core

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/dwago"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/common-components-go/pkg/pconst"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

type WagoEvent struct {
	Mac      string `json:"mac"`
	Consigne int    `json:"consigne"`
}

type WagoErrorEvent struct {
	Mac string `json:"mac"`
}

//ToJSON dump struct in json
func (driver WagoErrorEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToWagoErrorEvent convert interface to Blind object
func ToWagoErrorEvent(val interface{}) (*WagoErrorEvent, error) {
	var driver WagoErrorEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

//ToJSON dump struct in json
func (driver WagoEvent) ToJSON() (string, error) {
	inrec, err := json.Marshal(driver)
	if err != nil {
		return "", err
	}
	return string(inrec), err
}

//ToWagoEvent convert interface to nano object
func ToWagoEvent(val interface{}) (*WagoEvent, error) {
	var driver WagoEvent
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(inrec, &driver)
	return &driver, err
}

func (s *Service) sendInvalidWagoStatus(driver dwago.Wago) {
	url := "/read/groups/error/wago"
	evt := WagoErrorEvent{
		Mac: driver.Mac,
	}
	dump, _ := evt.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) removeWago(mac string) {
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	s.db.DeleteRecord(pconst.DbConfig, pconst.TbWagos, criteria)

	_, ok := s.wagos.Get(mac)
	if !ok {
		return
	}

	isConfigured := false
	remove := dwago.WagoDef{
		Mac:          mac,
		IsConfigured: &isConfigured,
	}
	s.wagos.Remove(mac)
	s.sendWagoUpdate(remove)
	s.driversSeen.Remove(mac)
}

func (s *Service) sendWagoSetup(driver dwago.WagoDef) {
	if driver.Mac == "" {
		return
	}
	url := "/write/wago/" + driver.Mac + "/" + pconst.UrlSetup
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) prepareWagoSetup(driver dwago.WagoDef) {
	err := database.SaveWagoConfig(s.db, driver)
	if err != nil {
		rlog.Error("Cannot update database", err.Error())
	}
	l, ok := s.wagos.Get(driver.Mac)
	sendSetup := false
	if !ok {
		sendSetup = true
	} else {
		wagoDef := l.(dwago.Wago)
		if !wagoDef.IsConfigured {
			sendSetup = true
		}
	}
	if sendSetup {
		s.sendWagoSetup(driver)
	}
}

func (s *Service) sendWagoUpdate(driver dwago.WagoDef) {
	if driver.Mac == "" {
		return
	}
	url := "/write/wago/" + driver.Mac + "/" + pconst.UrlSetting
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}

func (s *Service) updateWagoStatus(driver dwago.Wago) error {
	s.wagos.Set(driver.Mac, driver)
	return nil
}

func (s *Service) onWagoHello(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	var driver dwago.Wago
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	driver.Mac = strings.ToUpper(driver.Mac)
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	if driver.DumpFrequency == 0 {
		driver.DumpFrequency = 1000 //ms default value
	}

	err = s.updateWagoStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	cfg, _ := database.GetWagoConfig(s.db, driver.Mac)
	if cfg != nil {
		s.sendWagoSetup(*cfg)
	}
}

func (s *Service) onWagoStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Info(topic + " : " + string(msg.Payload()))
	var driver dwago.Wago
	err := json.Unmarshal(msg.Payload(), &driver)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	driver.Mac = strings.ToUpper(driver.Mac)
	s.driversSeen.Set(driver.Mac, time.Now().UTC())
	err = s.updateWagoStatus(driver)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
	if driver.Error == 0 {
		consigne := 0
		for _, cron := range driver.CronJobs {
			if cron.Action == "consigne" {
				consigne = cron.Status
				break
			}
		}
		url := "/read/groups/events/wago"
		evt := WagoEvent{
			Mac:      driver.Mac,
			Consigne: consigne,
		}
		dump, _ := evt.ToJSON()
		s.localSendCommand(url, dump)
	} else {
		s.sendInvalidWagoStatus(driver)
	}
}
