package core

import (
	"time"

	genericNetwork "github.com/energieip/common-components-go/pkg/network"
	"github.com/energieip/swh200-firmware-go/internal/database"
	"github.com/romana/rlog"
)

//ClusterNetwork network object
type ClusterNetwork struct {
	Iface genericNetwork.NetworkInterface
}

func (s *Service) createClusterNetwork() (ClusterNetwork, error) {
	broker, err := genericNetwork.NewNetwork(genericNetwork.MQTT)
	if err != nil {
		return ClusterNetwork{}, err
	}
	serverNet := ClusterNetwork{
		Iface: broker,
	}
	return serverNet, nil
}

func (s *Service) remoteClusterConnection(ip string, client ClusterNetwork) error {
	cbkServer := make(map[string]func(genericNetwork.Client, genericNetwork.Message))
	confServer := genericNetwork.NetworkConfig{
		IP:        ip,
		Port:      s.conf.NetworkBroker.Port,
		Callbacks: cbkServer,
		LogLevel:  s.conf.LogLevel,
		User:      s.conf.NetworkBroker.Login,
		Password:  s.conf.NetworkBroker.Password,
		CaPath:    s.conf.NetworkBroker.CaPath,
		Secure:    s.conf.NetworkBroker.Secure,
	}

	for {
		rlog.Info("Try to connect to " + ip)
		err := client.Iface.Initialize(confServer)
		if err == nil {
			rlog.Info("Connected to cluster server broker " + ip)
			return err
		}
		timer := time.NewTicker(time.Second)
		rlog.Error("Cannot connect to broker " + ip + " error: " + err.Error())
		rlog.Error("Try to reconnect " + ip + " in 1s")

		select {
		case <-timer.C:
			continue
		}
	}
}

func (s *Service) clusterDisconnect() {
	for _, cl := range s.cluster {
		cl.Iface.Disconnect()
	}
}

func (s *Service) clusterSendCommand(topic, content string) error {
	var res error
	for mac, cl := range s.cluster {
		err := cl.Iface.SendCommand(topic, content)
		if err != nil {
			rlog.Error("Error : " + err.Error() + " ; " + topic + " : " + content + " cluster:  " + mac)
			res = err
		} else {
			rlog.Debug(topic + " : " + content + " cluster: " + mac)
		}
	}
	return res
}

func (s *Service) removeClusterMember(mac string) error {
	val, ok := s.cluster[mac]
	if !ok {
		return nil
	}
	val.Iface.Disconnect()
	delete(s.cluster, mac)
	database.RemoveClusterConfig(s.db, mac)
	return nil
}
