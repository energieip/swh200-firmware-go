package core

import (
	"github.com/energieip/common-components-go/pkg/dwago"
	"github.com/energieip/common-components-go/pkg/pconst"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

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
	url := "/write/wago/" + driver.Mac + "/" + pconst.UrlSetting
	dump, _ := driver.ToJSON()
	s.localSendCommand(url, dump)
}
