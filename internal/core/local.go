package core

import (
	"github.com/energieip/common-components-go/pkg/dblind"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	genericNetwork "github.com/energieip/common-components-go/pkg/network"
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

func (s *Service) localConnection() error {
	cbkLocal := make(map[string]func(genericNetwork.Client, genericNetwork.Message))
	cbkLocal["/write/switch/commands"] = s.onSwitchCmd
	cbkLocal["/read/led/+/"+dl.UrlHello] = s.onLedHello
	cbkLocal["/read/led/+/"+dl.UrlStatus] = s.onLedStatus
	cbkLocal["/read/sensor/+/"+ds.UrlHello] = s.onSensorHello
	cbkLocal["/read/sensor/+/"+ds.UrlStatus] = s.onSensorStatus
	cbkLocal["/read/blind/+/"+dblind.UrlHello] = s.onBlindHello
	cbkLocal["/read/blind/+/"+dblind.UrlStatus] = s.onBlindStatus
	cbkLocal["/read/group/+/events/sensor"] = s.onGroupSensorEvent
	cbkLocal["/write/group/+/commands"] = s.onGroupCommand

	confLocal := genericNetwork.NetworkConfig{
		IP:               s.conf.LocalBroker.IP,
		Port:             s.conf.LocalBroker.Port,
		ClientName:       s.clientID,
		Callbacks:        cbkLocal,
		LogLevel:         s.conf.LogLevel,
		User:             s.conf.LocalBroker.Login,
		Password:         s.conf.LocalBroker.Password,
		ClientKey:        s.conf.LocalBroker.KeyPath,
		ServerCertificat: s.conf.LocalBroker.CaPath,
	}
	return s.local.Iface.Initialize(confLocal)
}

func (s *Service) localDisconnect() {
	s.local.Iface.Disconnect()
}

func (s *Service) localSendCommand(topic, content string) error {
	err := s.local.Iface.SendCommand(topic, content)
	if err != nil {
		rlog.Error("Cannot send : " + content + " on: " + topic + " Error: " + err.Error())
	} else {
		rlog.Info("Send : " + content + " on: " + topic)
	}
	return err
}
