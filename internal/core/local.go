package core

import (
	genericNetwork "github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/common-components-go/pkg/pconst"
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
	cbkLocal["/read/led/+/"+pconst.UrlHello] = s.onLedHello
	cbkLocal["/read/led/+/"+pconst.UrlStatus] = s.onLedStatus
	cbkLocal["/read/sensor/+/"+pconst.UrlHello] = s.onSensorHello
	cbkLocal["/read/sensor/+/"+pconst.UrlStatus] = s.onSensorStatus
	cbkLocal["/read/hvac/+/"+pconst.UrlHello] = s.onHvacHello
	cbkLocal["/read/hvac/+/"+pconst.UrlStatus] = s.onHvacStatus
	cbkLocal["/read/blind/+/"+pconst.UrlHello] = s.onBlindHello
	cbkLocal["/read/blind/+/"+pconst.UrlStatus] = s.onBlindStatus
	cbkLocal["/read/group/+/events/sensor"] = s.onGroupSensorEvent
	cbkLocal["/read/group/+/error/sensor"] = s.onGroupErrorEvent
	cbkLocal["/read/group/+/events/blind"] = s.onGroupBlindEvent
	cbkLocal["/read/group/+/error/blind"] = s.onGroupBlindErrorEvent
	cbkLocal["/read/group/+/events/nano"] = s.onGroupNanoEvent
	cbkLocal["/read/group/+/error/nano"] = s.onGroupNanoErrorEvent
	cbkLocal["/write/group/+/commands"] = s.onGroupCommand

	confLocal := genericNetwork.NetworkConfig{
		IP:        s.conf.LocalBroker.IP,
		Port:      s.conf.LocalBroker.Port,
		Callbacks: cbkLocal,
		LogLevel:  s.conf.LogLevel,
		User:      s.conf.LocalBroker.Login,
		Password:  s.conf.LocalBroker.Password,
		CaPath:    s.conf.LocalBroker.CaPath,
		Secure:    s.conf.LocalBroker.Secure,
	}
	return s.local.Iface.Initialize(confLocal)
}

func (s *Service) localDisconnect() {
	s.local.Iface.Disconnect()
}

func (s *Service) localSendCommand(topic, content string) error {
	err := s.local.Iface.SendCommand(topic, content)
	if err != nil {
		rlog.Error("Local cannot send : " + content + " on: " + topic + " Error: " + err.Error())
	} else {
		rlog.Debug("Local Sent : " + content + " on: " + topic)
	}
	return err
}
