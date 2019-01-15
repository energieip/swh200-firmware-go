package core

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/dblind"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/energieip/common-tools-go/pkg/tools"
	"github.com/romana/rlog"
)

const (
	ActionReload = "ReloadConfig"
	ActionSetup  = "Setup"
	ActionDump   = "DumpStatus"
	ActionRemove = "remove"

	UrlStatus = "status/dump"
	UrlHello  = "setup/hello"

	DefaultTimerDump = 10
)

//Service content
type Service struct {
	server                ServerNetwork  //Remote server
	local                 LocalNetwork   //local broker for drivers and services
	cluster               ClusterNetwork //Share broker in the cluster
	db                    Database
	mac                   string //Switch mac address
	events                chan string
	timerDump             time.Duration //in seconds
	ip                    string
	isConfigured          bool
	services              map[string]pkg.Service
	lastSystemUpgradeDate string
	friendlyName          string
	leds                  map[string]dl.Led
	sensors               map[string]ds.Sensor
	groups                map[int]Group
	blinds                map[string]dblind.Blind
}

//Initialize service
func (s *Service) Initialize(confFile string) error {
	hostname, _ := os.Hostname()
	clientID := "Switch" + hostname
	s.events = make(chan string)
	s.leds = make(map[string]dl.Led)
	s.sensors = make(map[string]ds.Sensor)
	s.blinds = make(map[string]dblind.Blind)
	s.groups = make(map[int]Group)

	conf, err := pkg.ReadServiceConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}

	mac, ip := tools.GetNetworkInfo()
	s.ip = ip
	s.mac = strings.ToUpper(mac[9:])
	s.groups = make(map[int]Group, 0)
	s.services = make(map[string]pkg.Service)

	os.Setenv("RLOG_LOG_LEVEL", conf.LogLevel)
	os.Setenv("RLOG_LOG_NOTIME", "yes")
	rlog.UpdateEnv()
	rlog.Info("Starting SwitchCore service")

	s.timerDump = DefaultTimerDump
	s.friendlyName = s.mac

	err = s.connectDatabase(conf.DB.ClientIP, conf.DB.ClientPort)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return err
	}

	err = s.createServerNetwork()
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}

	err = s.createLocalNetwork()
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.NetworkBroker.IP + " error: " + err.Error())
		return err
	}

	err = s.localConnection(*conf, clientID)
	if err != nil {
		rlog.Error("Cannot connect to drivers broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}

	err = s.createClusterNetwork()
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.NetworkBroker.IP + " error: " + err.Error())
		return err
	}
	go s.remoteClusterConnection(*conf, clientID+"group")

	go s.remoteServerConnection(*conf, clientID)
	rlog.Info("SwitchCore service started")
	return nil
}

//Stop service
func (s *Service) Stop() {
	rlog.Info("Stopping SwitchCore service")
	s.localDisconnect()
	s.serverDisconnect()
	s.clusterDisconnect()
	s.dbClose()
	rlog.Info("SwitchCore service stopped")
}

func (s *Service) sendHello() {

	switchDump := sd.Switch{
		Mac:           s.mac,
		IP:            s.ip,
		IsConfigured:  &s.isConfigured,
		Protocol:      "MQTTS",
		FriendlyName:  s.friendlyName,
		DumpFrequency: DefaultTimerDump,
	}
	dump, err := switchDump.ToJSON()
	if err != nil {
		rlog.Errorf("Could not dump switch %v status %v", s.mac, err.Error())
		return
	}

	err = s.serverSendCommand("/read/switch/"+switchDump.Mac+"/"+UrlHello, dump)
	if err != nil {
		rlog.Errorf("Could not send hello to the server %v status %v", s.mac, err.Error())
		return
	}
	rlog.Infof("Hello %v sent to the server", s.mac)
}

func (s *Service) sendDump() {
	status := sd.SwitchStatus{}
	status.Mac = s.mac
	status.Protocol = "MQTTS"
	status.IP = s.ip
	status.IsConfigured = &s.isConfigured
	status.FriendlyName = s.friendlyName
	services := make(map[string]pkg.ServiceStatus)

	for _, c := range s.services {
		component := pkg.ServiceStatus{}
		component.Name = c.Name
		component.PackageName = c.PackageName
		component.Version = c.Version
		status := component.GetServiceStatus()
		component.Status = &status
		services[component.Name] = component
	}

	status.Services = services
	status.Leds = s.getStatusLeds()
	status.Sensors = s.getStatusSensors()
	status.Blinds = s.getStatusBlinds()
	status.Groups = s.getStatusGroup()

	dump, err := status.ToJSON()
	if err != nil {
		rlog.Error("Could not dump switch status ", err.Error())
		return
	}

	err = s.serverSendCommand("/read/switch/"+s.mac+"/"+UrlStatus, dump)
	if err != nil {
		rlog.Errorf("Could not dump switch %v status %v", s.mac, err.Error())
		return
	}
	rlog.Infof("Status %v sent to the server", s.mac)
}

func (s *Service) updateConfiguration(switchConfig sd.SwitchConfig) {
	for _, led := range switchConfig.LedsSetup {
		s.prepareLedSetup(led)
	}
	for _, led := range switchConfig.LedsConfig {
		s.updateLedConfig(led)
	}

	for _, sensor := range switchConfig.SensorsSetup {
		s.prepareSensorSetup(sensor)
	}

	for _, sensor := range switchConfig.SensorsConfig {
		s.updateSensorConfig(sensor)
	}

	for _, blind := range switchConfig.BlindsSetup {
		s.prepareBlindSetup(blind)
	}

	for _, blind := range switchConfig.BlindsConfig {
		s.updateBlindConfig(blind)
	}

	for grID, group := range switchConfig.Groups {
		if _, ok := s.groups[grID]; !ok {
			rlog.Info("Group " + strconv.Itoa(grID) + " create it")
			s.createGroup(group)
			continue
		}
		rlog.Info("Group " + strconv.Itoa(grID) + " reload it")
		s.reloadGroupConfig(grID, group)
	}
}

func (s *Service) removeConfiguration(switchConfig sd.SwitchConfig) {
	for grID := range switchConfig.Groups {
		if group, ok := s.groups[grID]; ok {
			s.deleteGroup(group.Runtime)
		}
	}

	for ledMac := range switchConfig.LedsConfig {
		s.removeLed(ledMac)
	}

	for sensorMac := range switchConfig.SensorsConfig {
		s.removeSensor(sensorMac)
	}

	for blindMac := range switchConfig.BlindsConfig {
		s.removeBlind(blindMac)
	}
}

func (s *Service) cronDump() {
	timerDump := time.NewTicker(s.timerDump * time.Second)
	for {
		select {
		case <-timerDump.C:
			// s.events <- ActionDump
			if s.isConfigured {
				s.sendDump()
			} else {
				s.sendHello()
			}
		}
	}
}

func (s *Service) packagesInstall(switchConfig sd.SwitchConfig) {
	for name, service := range switchConfig.Services {
		if currentState, ok := s.services[name]; ok {
			if currentState.Version == service.Version {
				rlog.Info("Package " + name + " already in version " + service.Version + " skip it")
				continue
			}
		}
		rlog.Info("Install " + name + " in version " + service.Version)
		service.Install()
		version := pkg.GetPackageVersion(service.PackageName)
		if version != nil {
			service.Version = *version
		}
		s.services[service.Name] = service
	}
}

func (s *Service) packagesRemove(switchConfig sd.SwitchConfig) {
	pkg.RemoveServices(switchConfig.Services)
	for _, service := range switchConfig.Services {
		if _, ok := s.services[service.Name]; ok {
			delete(s.services, service.Name)
		}
	}
}

func (s *Service) systemUpdate(switchConfig sd.SwitchConfig) {
	SystemUpgrade()
}

//Run service mainloop
func (s *Service) Run() error {
	s.sendHello()
	go s.cronDump()
	for {
		select {
		case serverEvents := <-s.server.Events:
			for eventType, event := range serverEvents {
				switch eventType {
				case EventServerReload:
					if event.IsConfigured != nil {
						s.isConfigured = *event.IsConfigured
						s.friendlyName = s.mac
						s.timerDump = DefaultTimerDump
					}
					if !s.isConfigured {
						//a reset is performed
						continue
					}
					s.friendlyName = event.FriendlyName
					s.updateConfiguration(event)
					s.isConfigured = true

				case EventServerSetup:
					s.isConfigured = true
					s.friendlyName = event.FriendlyName
					s.systemUpdate(event)
					s.packagesInstall(event)
					// s.updateConfiguration(event)

				case EventServerRemove:
					if !s.isConfigured {
						//a reset is performed
						continue
					}
					s.packagesRemove(event)
					s.removeConfiguration(event)
				}
			}
		}
	}
	return nil
}
