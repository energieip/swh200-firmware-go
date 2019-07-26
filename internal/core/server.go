package core

import (
	"encoding/json"
	"strings"
	"time"

	sd "github.com/energieip/common-components-go/pkg/dswitch"
	genericNetwork "github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

const (
	EventServerSetup  = "serverSetup"
	EventServerReload = "serverReload"
	EventServerRemove = "serverRemove"
)

//ServerNetwork network object
type ServerNetwork struct {
	Iface  genericNetwork.NetworkInterface
	Events chan map[string]sd.SwitchConfig
}

func (s *Service) createServerNetwork() error {
	serverBroker, err := genericNetwork.NewNetwork(genericNetwork.MQTT)
	if err != nil {
		return err
	}
	serverNet := ServerNetwork{
		Iface:  serverBroker,
		Events: make(chan map[string]sd.SwitchConfig),
	}
	s.server = serverNet
	return nil
}

//RemoteServerConnection connect service to server broker
func (s *Service) remoteServerConnection() error {
	cbkServer := make(map[string]func(genericNetwork.Client, genericNetwork.Message))
	cbkServer["/write/switch/"+s.mac+"/setup/config"] = s.onSetup
	cbkServer["/write/switch/"+s.mac+"/update/settings"] = s.onUpdateSetting
	cbkServer["/remove/switch/"+s.mac+"/update/settings"] = s.onRemoveSetting

	confServer := genericNetwork.NetworkConfig{
		IP:        s.conf.NetworkBroker.IP,
		Port:      s.conf.NetworkBroker.Port,
		Callbacks: cbkServer,
		LogLevel:  s.conf.LogLevel,
		User:      s.conf.NetworkBroker.Login,
		Password:  s.conf.NetworkBroker.Password,
		Secure:    s.conf.NetworkBroker.Secure,
		CaPath:    s.conf.NetworkBroker.CaPath,
	}

	for {
		rlog.Info("Try to connect to " + s.conf.NetworkBroker.IP)
		err := s.server.Iface.Initialize(confServer)
		if err == nil {
			rlog.Info("Connected to server broker " + s.conf.NetworkBroker.IP)
			return err
		}
		timer := time.NewTicker(time.Second)
		rlog.Error("Cannot connect to broker " + s.conf.NetworkBroker.IP + " error: " + err.Error())
		rlog.Error("Try to reconnect " + s.conf.NetworkBroker.IP + " in 1s")

		select {
		case <-timer.C:
			continue
		}
	}
}

func (s *Service) onSetup(client genericNetwork.Client, msg genericNetwork.Message) {
	payload := msg.Payload()
	rlog.Info(msg.Topic() + " : " + string(payload))
	var switchConf sd.SwitchConfig
	err := json.Unmarshal(payload, &switchConf)
	if err != nil {
		rlog.Error("Cannot parse config ", err.Error())
		return
	}
	switchConf.Mac = strings.ToUpper(switchConf.Mac)
	event := make(map[string]sd.SwitchConfig)
	event[EventServerSetup] = switchConf
	s.server.Events <- event
}

func (s *Service) onRemoveSetting(client genericNetwork.Client, msg genericNetwork.Message) {
	payload := msg.Payload()
	rlog.Info(msg.Topic() + " : " + string(payload))
	var switchConf sd.SwitchConfig
	err := json.Unmarshal(payload, &switchConf)
	if err != nil {
		rlog.Error("Cannot parse config ", err.Error())
		return
	}
	switchConf.Mac = strings.ToUpper(switchConf.Mac)
	event := make(map[string]sd.SwitchConfig)
	event[EventServerRemove] = switchConf
	s.server.Events <- event
}

func (s *Service) onUpdateSetting(client genericNetwork.Client, msg genericNetwork.Message) {
	payload := msg.Payload()
	rlog.Info(msg.Topic() + " : " + string(payload))
	var switchConf sd.SwitchConfig
	err := json.Unmarshal(payload, &switchConf)
	if err != nil {
		rlog.Error("Cannot parse config ", err.Error())
		return
	}
	switchConf.Mac = strings.ToUpper(switchConf.Mac)
	event := make(map[string]sd.SwitchConfig)
	event[EventServerReload] = switchConf
	s.server.Events <- event
}

func (s *Service) serverDisconnect() {
	s.server.Iface.Disconnect()
}

func (s *Service) serverSendCommand(topic, content string) error {
	err := s.server.Iface.SendCommand(topic, content)
	if err != nil {
		rlog.Error("Server Cannot send : " + content + " on: " + topic + " Error: " + err.Error())
	} else {
		rlog.Debug("Server sent : " + content + " on: " + topic)
	}
	return err
}
