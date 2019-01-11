package core

import (
	"time"

	genericNetwork "github.com/energieip/common-components-go/pkg/network"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/romana/rlog"
)

//ClusterNetwork network object
type ClusterNetwork struct {
	Iface genericNetwork.NetworkInterface
}

func (s *Service) createClusterNetwork() error {
	broker, err := genericNetwork.NewNetwork(genericNetwork.MQTT)
	if err != nil {
		return err
	}
	serverNet := ClusterNetwork{
		Iface: broker,
	}
	s.cluster = serverNet
	return nil
}

func (s *Service) remoteClusterConnection(conf pkg.ServiceConfig, clientID string) error {
	cbkServer := make(map[string]func(genericNetwork.Client, genericNetwork.Message))
	cbkServer["/read/group/+/events/sensor"] = s.onGroupSensorEvent
	//TODO fix here cluster connection
	confServer := genericNetwork.NetworkConfig{
		IP:               conf.NetworkBroker.IP,
		Port:             conf.NetworkBroker.Port,
		ClientName:       clientID,
		Callbacks:        cbkServer,
		LogLevel:         conf.LogLevel,
		User:             conf.NetworkBroker.Login,
		Password:         conf.NetworkBroker.Password,
		ClientKey:        conf.NetworkBroker.KeyPath,
		ServerCertificat: conf.NetworkBroker.CaPath,
	}

	for {
		rlog.Info("Try to connect to " + conf.NetworkBroker.IP)
		err := s.cluster.Iface.Initialize(confServer)
		if err == nil {
			rlog.Info(clientID + " connected to server broker " + conf.NetworkBroker.IP)
			return err
		}
		timer := time.NewTicker(time.Second)
		rlog.Error("Cannot connect to broker " + conf.NetworkBroker.IP + " error: " + err.Error())
		rlog.Error("Try to reconnect " + conf.NetworkBroker.IP + " in 1s")

		select {
		case <-timer.C:
			continue
		}
	}
}

func (s *Service) clusterDisconnect() {
	s.cluster.Iface.Disconnect()
}

func (s *Service) clusterSendCommand(topic, content string) error {
	return s.cluster.Iface.SendCommand(topic, content)
}
