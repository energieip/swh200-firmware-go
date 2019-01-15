package core

import (
	"github.com/energieip/common-components-go/pkg/dblind"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	genericNetwork "github.com/energieip/common-components-go/pkg/network"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/romana/rlog"
)

//LocalNetwork network object
type LocalNetwork struct {
	Iface genericNetwork.NetworkInterface
}

func (s *Service) createLocalNetwork() error {
	driverBroker, err := genericNetwork.NewNetwork(genericNetwork.MQTT)
	if err != nil {
		return err
	}
	driversNet := LocalNetwork{
		Iface: driverBroker,
	}
	s.local = driversNet
	return nil

}

func (s *Service) localConnection(conf pkg.ServiceConfig, clientID string) error {
	cbkLocal := make(map[string]func(genericNetwork.Client, genericNetwork.Message))
	cbkLocal["/read/led/+/"+dl.UrlHello] = s.onLedHello
	cbkLocal["/read/led/+/"+dl.UrlStatus] = s.onLedStatus
	cbkLocal["/read/sensor/+/"+ds.UrlHello] = s.onSensorHello
	cbkLocal["/read/sensor/+/"+ds.UrlStatus] = s.onSensorStatus
	cbkLocal["/read/blind/+/"+dblind.UrlHello] = s.onBlindHello
	cbkLocal["/read/blind/+/"+dblind.UrlStatus] = s.onBlindStatus
	confLocal := genericNetwork.NetworkConfig{
		IP:               conf.LocalBroker.IP,
		Port:             conf.LocalBroker.Port,
		ClientName:       clientID,
		Callbacks:        cbkLocal,
		LogLevel:         conf.LogLevel,
		User:             conf.LocalBroker.Login,
		Password:         conf.LocalBroker.Password,
		ClientKey:        conf.LocalBroker.KeyPath,
		ServerCertificat: conf.LocalBroker.CaPath,
	}
	return s.local.Iface.Initialize(confLocal)
}

func (s *Service) localDisconnect() {
	s.local.Iface.Disconnect()
}

func (s *Service) localSendCommand(topic, content string) error {
	err := s.local.Iface.SendCommand(topic, content)
	if err != nil {
		rlog.Error("Cannot send : " + content + "on: " + topic + " Error: " + err.Error())
	} else {
		rlog.Info("Send : " + content + "on: " + topic)
	}
	return err
}
