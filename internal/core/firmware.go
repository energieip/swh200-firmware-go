package core

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/swh200-firmware-go/internal/api"
	"github.com/energieip/swh200-firmware-go/internal/database"

	"github.com/energieip/common-components-go/pkg/dblind"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/energieip/common-components-go/pkg/tools"
	"github.com/romana/rlog"
)

const (
	ActionReload = "ReloadConfig"
	ActionSetup  = "Setup"
	ActionDump   = "DumpStatus"
	ActionRemove = "remove"

	UrlStatus = "status/dump"
	UrlHello  = "setup/hello"

	DefaultTimerDump = 1000
)

//Service content
type Service struct {
	server                ServerNetwork             //Remote server
	local                 LocalNetwork              //local broker for drivers and services
	cluster               map[string]ClusterNetwork //Share broker in the cluster
	db                    database.Database
	mac                   string //Switch mac address
	events                chan string
	timerDump             time.Duration //in seconds
	ip                    string
	isConfigured          bool
	services              map[string]pkg.Service
	lastSystemUpgradeDate string
	friendlyName          string
	leds                  map[string]dl.Led
	ledsToAuto            map[string]*int
	sensors               map[string]ds.Sensor
	groups                map[int]Group
	blinds                map[string]dblind.Blind
	conf                  pkg.ServiceConfig
	clientID              string
	driversSeen           map[string]time.Time
	api                   *api.API
}

//Initialize service
func (s *Service) Initialize(confFile string) error {
	hostname, _ := os.Hostname()
	s.clientID = "Switch" + hostname
	s.events = make(chan string)
	s.leds = make(map[string]dl.Led)
	s.ledsToAuto = make(map[string]*int)
	s.sensors = make(map[string]ds.Sensor)
	s.blinds = make(map[string]dblind.Blind)
	s.groups = make(map[int]Group)
	s.cluster = make(map[string]ClusterNetwork)
	s.driversSeen = make(map[string]time.Time)

	conf, err := pkg.ReadServiceConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}
	s.conf = *conf

	mac, ip := tools.GetNetworkInfo()
	s.ip = ip
	s.mac = strings.ToUpper(mac[9:])
	s.services = make(map[string]pkg.Service)

	os.Setenv("RLOG_LOG_LEVEL", conf.LogLevel)
	os.Setenv("RLOG_LOG_NOTIME", "yes")
	os.Setenv("RLOG_TIME_FORMAT", "2006/01/06 15:04:05.000")
	rlog.UpdateEnv()
	rlog.Info("Starting SwitchCore service")

	s.timerDump = DefaultTimerDump
	s.friendlyName = s.mac

	db, err := database.ConnectDatabase(conf.DB.ClientIP, conf.DB.ClientPort)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return err
	}
	s.db = db

	groups := database.GetGroupsConfig(s.db)
	rlog.Info("=== get groups ", groups)
	for grID, group := range groups {
		rlog.Info("Restore group ", grID)
		s.createGroup(group)
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

	err = s.localConnection()
	if err != nil {
		rlog.Error("Cannot connect to drivers broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}

	clusters := database.GetClusterConfig(s.db)
	for _, cl := range clusters {
		client, err := s.createClusterNetwork()
		if err != nil {
			rlog.Warn("Cannot create a connection to", cl.Mac, err.Error())
			continue
		}
		s.cluster[cl.Mac] = client
		go s.remoteClusterConnection(cl.IP, client)
	}

	go s.remoteServerConnection()
	web := api.InitAPI(s.db, *conf)
	s.api = web
	rlog.Info("SwitchCore service started")
	return nil
}

//Stop service
func (s *Service) Stop() {
	rlog.Info("Stopping SwitchCore service")
	s.localDisconnect()
	s.serverDisconnect()
	s.clusterDisconnect()
	database.DBClose(s.db)
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
	rlog.Info("Hello" + s.mac + " sent to the server")
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

	clusters := make(map[string]sd.SwitchCluster)
	for _, cl := range database.GetClusterConfig(s.db) {
		clusters[cl.Mac] = cl
	}
	status.ClusterBroker = clusters

	status.Services = services
	timeNow := time.Now().UTC()
	leds := database.GetStatusLeds(s.db)
	sensors := database.GetStatusSensors(s.db)
	blinds := database.GetStatusBlinds(s.db)

	dumpSensors := make(map[string]ds.Sensor)
	dumpLeds := make(map[string]dl.Led)
	dumpBlinds := make(map[string]dblind.Blind)
	for _, driver := range leds {
		val, ok := s.driversSeen[driver.Mac]
		if ok {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val) <= maxDuration {
				dumpLeds[driver.Mac] = driver
				continue
			} else {
				rlog.Warn("LED " + driver.Mac + " no longer seen; drop it")
				delete(s.leds, driver.Mac)
				delete(s.driversSeen, driver.Mac)
				_, ok := s.ledsToAuto[driver.Mac]
				if ok {
					delete(s.ledsToAuto, driver.Mac)
				}
				database.RemoveLedStatus(s.db, driver.Mac)
			}
		}
	}
	status.Leds = dumpLeds

	for _, driver := range sensors {
		val, ok := s.driversSeen[driver.Mac]
		if ok {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val) <= maxDuration {
				dumpSensors[driver.Mac] = driver
				continue
			} else {
				rlog.Warn("Sensor " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidStatus(driver)
				delete(s.sensors, driver.Mac)
				delete(s.driversSeen, driver.Mac)
				database.RemoveSensorStatus(s.db, driver.Mac)
			}
		}
	}
	status.Sensors = dumpSensors

	for _, driver := range blinds {
		val, ok := s.driversSeen[driver.Mac]
		if ok {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val) <= maxDuration {
				dumpBlinds[driver.Mac] = driver
				continue
			} else {
				rlog.Warn("Blind " + driver.Mac + " no longer seen; drop it")
				delete(s.blinds, driver.Mac)
				delete(s.driversSeen, driver.Mac)
				database.RemoveBlindStatus(s.db, driver.Mac)
			}
		}
	}
	status.Blinds = dumpBlinds
	status.Groups = database.GetStatusGroup(s.db)

	dump, _ := status.ToJSON()
	s.serverSendCommand("/read/switch/"+s.mac+"/"+UrlStatus, dump)
}

func (s *Service) updateConfiguration(switchConfig sd.SwitchConfig) {
	s.timerDump = time.Duration(switchConfig.DumpFrequency)
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
		database.UpdateGroupConfig(s.db, group)
		if _, ok := s.groups[grID]; !ok {
			rlog.Info("Group " + strconv.Itoa(grID) + " create it")
			s.createGroup(group)
			continue
		}
		rlog.Info("Group " + strconv.Itoa(grID) + " reload it")
		s.reloadGroupConfig(grID, group)
	}

	if len(switchConfig.ClusterBroker) > 0 {
		database.UpdateClusterConfig(s.db, switchConfig.ClusterBroker)
		for _, cl := range switchConfig.ClusterBroker {
			client, err := s.createClusterNetwork()
			if err != nil {
				rlog.Warn("Cannot create a connection to", cl.Mac, err.Error())
				continue
			}
			s.cluster[cl.Mac] = client
			go s.remoteClusterConnection(cl.IP, client)
		}
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

	for mac := range switchConfig.ClusterBroker {
		s.removeClusterMember(mac)
	}
}

func (s *Service) cronDump() {
	timerDump := time.NewTicker(s.timerDump * time.Millisecond)
	for {
		select {
		case <-timerDump.C:
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
	go s.cronLedMode()
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
						if !s.isConfigured {
							rlog.Warn("Received Reset: Stop group and reset database")
							for _, group := range s.groups {
								s.deleteGroup(group.Runtime)
							}
							s.leds = make(map[string]dl.Led)
							s.ledsToAuto = make(map[string]*int)
							s.sensors = make(map[string]ds.Sensor)
							s.blinds = make(map[string]dblind.Blind)
							s.groups = make(map[int]Group)
							for mac := range s.cluster {
								s.removeClusterMember(mac)
							}
							database.ResetDB(s.db)
						}
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
					// s.systemUpdate(event)
					// s.packagesInstall(event)
					// s.updateConfiguration(event)

				case EventServerRemove:
					if !s.isConfigured {
						//a reset is performed
						continue
					}
					// s.packagesRemove(event)
					s.removeConfiguration(event)
				}
			}
		}
	}
	return nil
}
