package core

import (
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/dgroup"

	"github.com/energieip/common-components-go/pkg/dnanosense"
	"github.com/energieip/common-components-go/pkg/dswitch"

	"github.com/energieip/common-components-go/pkg/dwago"

	"github.com/energieip/common-components-go/pkg/pconst"

	"github.com/energieip/swh200-firmware-go/internal/api"
	"github.com/energieip/swh200-firmware-go/internal/database"

	"github.com/energieip/common-components-go/pkg/dblind"
	"github.com/energieip/common-components-go/pkg/dhvac"
	dl "github.com/energieip/common-components-go/pkg/dled"
	ds "github.com/energieip/common-components-go/pkg/dsensor"
	sd "github.com/energieip/common-components-go/pkg/dswitch"
	pkg "github.com/energieip/common-components-go/pkg/service"
	"github.com/energieip/common-components-go/pkg/tools"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/romana/rlog"
)

const (
	ActionReload = "ReloadConfig"
	ActionSetup  = "Setup"
	ActionDump   = "DumpStatus"
	ActionRemove = "remove"

	UrlStatus = "status/dump"
	UrlHello  = "setup/hello"

	DefaultTimerDump = 10000
)

//Service content
type Service struct {
	server                ServerNetwork             //Remote server
	local                 LocalNetwork              //local broker for drivers and services
	cluster               map[string]ClusterNetwork //Share broker in the cluster
	clusterID             int
	profil                string
	db                    database.Database
	mac                   string //Switch mac address
	label                 string
	events                chan string
	timerDump             time.Duration //in seconds
	ip                    string
	isConfigured          bool
	services              map[string]pkg.Service
	lastSystemUpgradeDate string
	friendlyName          string
	leds                  cmap.ConcurrentMap
	ledsToAuto            map[string]*int
	sensors               cmap.ConcurrentMap
	groups                map[int]Group
	blinds                cmap.ConcurrentMap
	nanos                 cmap.ConcurrentMap
	wagos                 cmap.ConcurrentMap
	hvacs                 cmap.ConcurrentMap
	groupStatus           cmap.ConcurrentMap
	conf                  pkg.ServiceConfig
	driversSeen           cmap.ConcurrentMap
	api                   *api.API
	consumption           *dswitch.SwitchConsumptions
	expectedLeds          int //for BAES control
}

//Initialize service
func (s *Service) Initialize(confFile string) error {
	s.events = make(chan string)
	s.leds = cmap.New()
	s.ledsToAuto = make(map[string]*int)
	s.sensors = cmap.New()
	s.blinds = cmap.New()
	s.hvacs = cmap.New()
	s.nanos = cmap.New()
	s.wagos = cmap.New()
	s.groupStatus = cmap.New()
	s.groups = make(map[int]Group)
	s.cluster = make(map[string]ClusterNetwork)
	s.driversSeen = cmap.New()
	conso := dswitch.SwitchConsumptions{}
	s.consumption = &conso

	conf, err := pkg.ReadServiceConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}
	s.conf = *conf

	mac, ip := tools.GetNetworkInfo()
	s.ip = ip
	s.mac = strings.ToUpper(mac)
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
	for grID, group := range groups {
		rlog.Info("Restore group ", grID)
		s.createGroup(group)
	}
	leds := database.GetLedsConfig(s.db)
	s.expectedLeds = len(leds)

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

	wagos := database.GetWagosConfig(s.db)
	for _, wago := range wagos {
		if wago.Mac != "" {
			dump, _ := wago.ToJSON()
			err := s.localSendCommand("/write/wago/"+wago.Mac+"/"+pconst.UrlSetup, dump)
			if err != nil {
				rlog.Errorf("Could not send setup to the modbus2mqtt service %v status %v", wago.Mac, err.Error())
			}
		}
	}

	go s.remoteServerConnection()
	web := api.InitAPI(s.db, *conf, s.consumption)
	s.api = web
	rlog.Info("SwitchCore service started")
	go s.activateGPIOs()
	go s.baesManagement()
	go s.cronCheckNetwork()
	return nil
}

func (s *Service) cronCheckNetwork() {
	timerDump := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-timerDump.C:
			count := s.leds.Count() + s.sensors.Count() + s.blinds.Count() + s.hvacs.Count()
			if count == 0 {
				rlog.Info("No driver seen: reboot")
				cmd := exec.Command("reboot")
				cmd.Run()
			}
			timerDump.Stop()
			timerDump = time.NewTicker(5 * time.Minute)
		}
	}
}

func (s *Service) baesManagement() {
	min := (s.expectedLeds * 90) / 100
	for {
		time.Sleep(5 * time.Second)
		if s.leds.Count() >= min {
			rlog.Info("Switch is ready switch BAES off")
			cmd := exec.Command("gpio", "write", "43", "1")
			_, err := cmd.CombinedOutput()
			if err != nil {
				rlog.Error("gpio write 43 1 update finished with " + err.Error())
				return
			}
			break
		}
	}
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
		Label:         &s.label,
		IP:            s.ip,
		IsConfigured:  &s.isConfigured,
		Protocol:      "MQTTS",
		FriendlyName:  s.friendlyName,
		DumpFrequency: DefaultTimerDump,
	}
	dump, _ := switchDump.ToJSON()

	err := s.serverSendCommand("/read/switch/"+switchDump.Mac+"/"+UrlHello, dump)
	if err != nil {
		rlog.Errorf("Could not send hello to the server %v status %v", s.mac, err.Error())
		return
	}
	rlog.Info("Hello " + s.mac + " sent to the server")
}

func (s *Service) sendDump() {
	ledsPower := int64(0)
	blindsPower := int64(0)
	hvacsPower := int64(0)
	totalPower := int64(0)

	status := sd.SwitchStatus{}
	status.Mac = s.mac
	status.DumpFrequency = int(s.timerDump)
	status.Cluster = s.clusterID
	status.Label = &s.label
	status.Profil = s.profil
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

	dumpSensors := make(map[string]ds.Sensor)
	dumpLeds := make(map[string]dl.Led)
	dumpBlinds := make(map[string]dblind.Blind)
	dumpHvacs := make(map[string]dhvac.Hvac)
	dumpWagos := make(map[string]dwago.Wago)
	dumpNanos := make(map[string]dnanosense.Nanosense)
	dumpGroups := make(map[int]dgroup.GroupStatus)
	for _, dr := range s.leds.Items() {
		driver, err := dl.ToLed(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpLeds[driver.Mac] = *driver
				ledsPower += int64(driver.LinePower)
				totalPower += int64(driver.LinePower)
				continue
			} else {
				rlog.Warn("LED " + driver.Mac + " no longer seen; drop it")
				s.leds.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
				_, ok := s.ledsToAuto[driver.Mac]
				if ok {
					delete(s.ledsToAuto, driver.Mac)
				}
			}
		} else {
			_, ok := s.leds.Get(driver.Mac)
			if ok {
				s.leds.Remove(driver.Mac)
			}
		}
	}
	status.Leds = dumpLeds

	for _, dr := range s.sensors.Items() {
		driver, err := ds.ToSensor(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpSensors[driver.Mac] = *driver
				continue
			} else {
				rlog.Warn("Sensor " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidStatus(*driver)
				s.sensors.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
			}
		} else {
			_, ok := s.sensors.Get(driver.Mac)
			if ok {
				s.sensors.Remove(driver.Mac)
			}
		}
	}
	status.Sensors = dumpSensors

	for _, dr := range s.blinds.Items() {
		driver, err := dblind.ToBlind(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpBlinds[driver.Mac] = *driver
				blindsPower += int64(driver.LinePower)
				totalPower += int64(driver.LinePower)
				continue
			} else {
				rlog.Warn("Blind " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidBlindStatus(*driver)
				s.blinds.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
			}
		} else {
			_, ok := s.blinds.Get(driver.Mac)
			if ok {
				s.blinds.Remove(driver.Mac)
			}
		}
	}
	status.Blinds = dumpBlinds

	for _, dr := range s.hvacs.Items() {
		driver, err := dhvac.ToHvac(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpHvacs[driver.Mac] = *driver
				hvacsPower += int64(driver.LinePower)
				totalPower += int64(driver.LinePower)
				continue
			} else {
				rlog.Warn("HVAC " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidHvacStatus(*driver)
				s.hvacs.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
			}
		} else {
			_, ok := s.hvacs.Get(driver.Mac)
			if ok {
				s.hvacs.Remove(driver.Mac)
			}
		}
	}
	status.Hvacs = dumpHvacs

	for _, dr := range s.wagos.Items() {
		driver, err := dwago.ToWago(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpWagos[driver.Mac] = *driver
				continue
			} else {
				rlog.Warn("WAGO " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidWagoStatus(*driver)
				s.wagos.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
			}
		} else {
			_, ok := s.wagos.Get(driver.Mac)
			if ok {
				s.wagos.Remove(driver.Mac)
			}
		}
	}
	status.Wagos = dumpWagos

	for _, dr := range s.nanos.Items() {
		driver, err := dnanosense.ToNanosense(dr)
		if err != nil {
			continue
		}
		val, ok := s.driversSeen.Get(driver.Mac)
		if ok && val != nil {
			maxDuration := time.Duration(5*driver.DumpFrequency) * time.Millisecond
			if timeNow.Sub(val.(time.Time)) <= maxDuration {
				dumpNanos[driver.Mac] = *driver
				continue
			} else {
				rlog.Warn("Nanos " + driver.Mac + " no longer seen; drop it")
				s.sendInvalidNanoStatus(*driver)
				s.nanos.Remove(driver.Mac)
				s.driversSeen.Remove(driver.Mac)
			}
		} else {
			_, ok := s.nanos.Get(driver.Mac)
			if ok {
				s.nanos.Remove(driver.Mac)
			}
		}
	}
	status.Nanos = dumpNanos

	go func() {
		wagos := database.GetWagosConfig(s.db)
		for _, wago := range wagos {
			if wago.Mac != "" {
				_, ok := s.wagos.Get(wago.Mac)
				if !ok {
					dump, _ := wago.ToJSON()
					err := s.localSendCommand("/write/wago/"+wago.Mac+"/"+pconst.UrlSetup, dump)
					if err != nil {
						rlog.Errorf("Could not send setup to the modbus2mqtt service %v status %v", wago.Mac, err.Error())
					}
				} else {
					if len(wago.Nanosenses) != s.nanos.Count() {
						dump, _ := wago.ToJSON()
						err := s.localSendCommand("/write/wago/"+wago.Mac+"/"+pconst.UrlSetup, dump)
						if err != nil {
							rlog.Errorf("Could not send setup to the modbus2mqtt service %v status %v", wago.Mac, err.Error())
						}
					}
				}
			}
		}
	}()

	for _, elt := range s.groupStatus.Items() {
		gr, err := dgroup.ToGroupStatus(elt)
		if err != nil {
			continue
		}
		dumpGroups[gr.Group] = *gr
	}
	status.Groups = dumpGroups
	status.BlindsPower = blindsPower
	status.HvacsPower = hvacsPower
	status.LedsPower = ledsPower
	status.TotalPower = totalPower

	gpioStatus := s.getGPIOStates()
	status.StateBaes = gpioStatus[0]
	status.StatePuls1 = gpioStatus[1]
	status.StatePuls2 = gpioStatus[2]
	status.StatePuls3 = gpioStatus[3]
	status.StatePuls4 = gpioStatus[4]
	status.StatePuls5 = gpioStatus[5]
	s.consumption.BlindPower = int(blindsPower)
	s.consumption.LightingPower = int(ledsPower)
	s.consumption.TotalPower = int(totalPower)
	s.consumption.HvacPower = int(hvacsPower)

	dump, _ := status.ToJSON()
	s.serverSendCommand("/read/switch/"+s.mac+"/"+UrlStatus, dump)
}

func (s *Service) updateConfiguration(switchConfig sd.SwitchConfig) {
	if switchConfig.DumpFrequency == 0 {
		switchConfig.DumpFrequency = DefaultTimerDump
	}
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

	for _, hvac := range switchConfig.HvacsSetup {
		s.prepareHvacSetup(hvac)
	}

	for _, hvac := range switchConfig.HvacsConfig {
		s.updateHvacConfig(hvac)
	}

	for _, wago := range switchConfig.WagosSetup {
		wagoDev := dwago.WagoDef{}
		wagoDev.Cluster = &wago.Cluster
		freq := 10000
		if wago.DumpFrequency != nil {
			freq = *wago.DumpFrequency
		}
		wagoDev.DumpFrequency = &freq
		wagoDev.FriendlyName = wago.FriendlyName
		wagoDev.IP = wago.IP
		wagoDev.Label = wago.Label
		wagoDev.Mac = wago.Mac
		wagoDev.IsConfigured = wago.IsConfigured
		wagoDev.ModbusOffset = wago.ModbusOffset
		var crons []dwago.CronJobDef
		for k, v := range wago.API {
			cron := dwago.CronJobDef{}
			cron.Action = k
			modID, _ := strconv.Atoi(v)
			cron.ModbusID = modID
			crons = append(crons, cron)
		}
		wagoDev.CronJobs = crons
		database.SaveWagoConfig(s.db, wagoDev)
	}

	for _, nano := range switchConfig.NanosSetup {
		wagoMac := strings.Split(nano.Mac, ".")[0]
		wago, _ := database.GetWagoConfig(s.db, wagoMac)
		if wago == nil {
			continue
		}
		modbusOffset := 0
		if wago.ModbusOffset != nil {
			modbusOffset = *wago.ModbusOffset
		}

		friend := nano.Label
		if nano.FriendlyName != nil {
			friend = *nano.FriendlyName
		}
		nanoDev := dnanosense.NanosenseDef{
			Cluster:      nano.Cluster,
			Group:        nano.Group,
			FriendlyName: friend,
			Label:        nano.Label,
			Mac:          nano.Mac,
		}
		covAPI, ok := nano.API["CoV"]
		if ok {
			i, err := strconv.Atoi(covAPI)
			if err == nil {
				nanoDev.COV = modbusOffset + ((nano.ModbusID - 1) * len(nano.API)) + (i - 1)
			}
		}
		co2API, ok := nano.API["CO2"]
		if ok {
			i, err := strconv.Atoi(co2API)
			if err == nil {
				nanoDev.CO2 = modbusOffset + ((nano.ModbusID - 1) * len(nano.API)) + (i - 1)
			}
		}
		HygoAPI, ok := nano.API["Hygro"]
		if ok {
			i, err := strconv.Atoi(HygoAPI)
			if err == nil {
				nanoDev.Hygrometry = modbusOffset + ((nano.ModbusID - 1) * len(nano.API)) + (i - 1)
			}
		}
		tempAPI, ok := nano.API["Temp"]
		if ok {
			i, err := strconv.Atoi(tempAPI)
			if err == nil {
				nanoDev.Temperature = modbusOffset + ((nano.ModbusID - 1) * len(nano.API)) + (i - 1)
			}
		}
		var nanos []dnanosense.NanosenseDef
		for _, n := range wago.Nanosenses {
			if n.Mac != nano.Mac {
				nanos = append(nanos, n)
			}
		}
		nanos = append(nanos, nanoDev)
		wago.Nanosenses = nanos
		database.SaveWagoConfig(s.db, *wago)
	}

	for _, user := range switchConfig.Users {
		database.SaveUserConfig(s.db, user)
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

	cluster := 0
	if switchConfig.Cluster != nil {
		cluster = *switchConfig.Cluster
	}
	elt := sd.SwitchDefinition{
		IP:            switchConfig.IP,
		FriendlyName:  switchConfig.FriendlyName,
		Label:         switchConfig.Label,
		Profil:        switchConfig.Profil,
		Cluster:       cluster,
		DumpFrequency: switchConfig.DumpFrequency,
	}
	s.updateIPConfig(switchConfig.IP, elt)
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

	for hvacMac := range switchConfig.HvacsConfig {
		s.removeHvac(hvacMac)
	}

	for mac := range switchConfig.WagosConfig {
		s.removeWago(mac)
	}

	for mac := range switchConfig.ClusterBroker {
		s.removeClusterMember(mac)
	}

	for user := range switchConfig.Users {
		database.RemoveUserConfig(s.db, user)
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
			timerDump.Stop()
			timerDump = time.NewTicker(s.timerDump * time.Millisecond)
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
	rand.Seed(time.Now().UTC().UnixNano())
	delay := rand.Int63n(50)
	time.Sleep(time.Duration(delay) * time.Second)
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
							s.label = ""
							s.profil = "none"
							s.leds = cmap.New()
							s.ledsToAuto = make(map[string]*int)
							s.sensors = cmap.New()
							s.blinds = cmap.New()
							s.hvacs = cmap.New()
							s.nanos = cmap.New()
							s.wagos = cmap.New()
							s.clusterID = 0
							s.driversSeen = cmap.New()
							s.groups = make(map[int]Group)
							for mac := range s.cluster {
								s.removeClusterMember(mac)
							}
							database.ResetDB(s.db)
							s.resetPSE()
						}
					}
					if !s.isConfigured {
						//a reset is performed
						continue
					}
					s.friendlyName = event.FriendlyName
					if event.Label != nil {
						s.label = *event.Label
					}
					if event.Cluster != nil {
						s.clusterID = *event.Cluster
					}
					if event.Profil != "" {
						s.profil = event.Profil
					}
					s.updateConfiguration(event)
					s.isConfigured = true

				case EventServerSetup:
					s.isConfigured = true
					s.friendlyName = event.FriendlyName
					if event.Label != nil {
						s.label = *event.Label
					}
					if event.Profil != "" {
						s.profil = event.Profil
					}
					if event.Cluster != nil {
						s.clusterID = *event.Cluster
					}
					// s.systemUpdate(event)
					// s.packagesInstall(event)
					s.updateConfiguration(event)

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
