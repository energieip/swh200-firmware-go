package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/pconst"

	"github.com/energieip/swh200-firmware-go/internal/database"

	"github.com/energieip/common-components-go/pkg/dgroup"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/network"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/romana/rlog"
)

const (
	EventChange = "change"
	EventStop   = "stop"
	EventManual = "manual"
	EventBlind  = "blind"
	EventHvac   = "hvac"
)

// Group logical
type Group struct {
	Event              chan map[string]*gm.GroupConfig
	Runtime            gm.GroupConfig
	Setpoint           int
	FirstDaySetpoint   int
	Brightness         int
	Presence           bool
	Opened             bool
	Slope              int
	TimeToAuto         int
	Scale              int    //brightness correction scale
	DbID               string //Database entry ID
	PresenceTimeout    int
	Error              int
	Sensors            cmap.ConcurrentMap
	SensorsIssue       cmap.ConcurrentMap
	Blinds             cmap.ConcurrentMap
	BlindsIssue        cmap.ConcurrentMap
	Nanosenses         cmap.ConcurrentMap
	NanosensesIssue    cmap.ConcurrentMap
	FirstDay           cmap.ConcurrentMap
	SetpointBlinds     *int
	SetpointSlatBlinds *int
	ShiftTemp          *int //in 1/10°C
	LastPresenceStatus bool
	Counter            int
	CeilingTemperature int
	CeilingHumidity    int
	Temperature        int
	Hygrometry         int
	CO2                int
	COV                int
}

func (s *Service) onGroupSensorEvent(client network.Client, msg network.Message) {
	rlog.Debug(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var sensor SensorEvent
	err = json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.Sensors.Set(sensor.Mac, sensor)
	_, ok = group.SensorsIssue.Get(sensor.Mac)
	if ok {
		//sensor no longer problematic
		group.SensorsIssue.Remove(sensor.Mac)
	}
}

func (s *Service) onGroupNanoEvent(client network.Client, msg network.Message) {
	rlog.Debug(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var nano NanoEvent
	err = json.Unmarshal(msg.Payload(), &nano)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.Nanosenses.Set(nano.Label, nano)
	_, ok = group.NanosensesIssue.Get(nano.Label)
	if ok {
		//sensor no longer problematic
		group.NanosensesIssue.Remove(nano.Label)
	}
}

func (s *Service) onGroupBlindEvent(client network.Client, msg network.Message) {
	rlog.Debug(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var blind BlindEvent
	err = json.Unmarshal(msg.Payload(), &blind)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.Blinds.Set(blind.Mac, blind)
	_, ok = group.BlindsIssue.Get(blind.Mac)
	if ok {
		//blind no longer problematic
		group.BlindsIssue.Remove(blind.Mac)
	}
}

func (s *Service) onGroupErrorEvent(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var sensor SensorErrorEvent
	err = json.Unmarshal(msg.Payload(), &sensor)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.SensorsIssue.Set(sensor.Mac, true)
}

func (s *Service) onGroupBlindErrorEvent(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var blind BlindErrorEvent
	err = json.Unmarshal(msg.Payload(), &blind)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.BlindsIssue.Set(blind.Mac, true)
}

func (s *Service) onGroupNanoErrorEvent(client network.Client, msg network.Message) {
	rlog.Info(msg.Topic() + " : " + string(msg.Payload()))
	sGrID := strings.Split(msg.Topic(), "/")[3]
	grID, err := strconv.Atoi(sGrID)
	if err != nil {
		return
	}

	group, ok := s.groups[grID]
	if !ok {
		rlog.Debug("Skip group")
		return
	}

	var nano NanoErrorEvent
	err = json.Unmarshal(msg.Payload(), &nano)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	group.NanosensesIssue.Set(nano.Label, true)
}

func (s *Service) dumpGroupStatus(group Group) error {
	name := ""
	if group.Runtime.FriendlyName != nil {
		name = *group.Runtime.FriendlyName
	}
	correctionInterval := 1
	if group.Runtime.CorrectionInterval != nil {
		correctionInterval = *group.Runtime.CorrectionInterval
	}
	auto := false
	if group.Runtime.Auto != nil {
		auto = *group.Runtime.Auto
	}
	slopeStartManual := 0
	slopeStopManual := 0
	slopeStartAuto := 0
	slopeStopAuto := 0

	if group.Runtime.SlopeStartAuto != nil {
		slopeStartAuto = *group.Runtime.SlopeStartAuto
	}
	if group.Runtime.SlopeStopAuto != nil {
		slopeStopAuto = *group.Runtime.SlopeStopAuto
	}
	if group.Runtime.SlopeStartManual != nil {
		slopeStartManual = *group.Runtime.SlopeStartManual
	}
	if group.Runtime.SlopeStopManual != nil {
		slopeStopManual = *group.Runtime.SlopeStopManual
	}

	sensorRule := gm.SensorAverage
	if group.Runtime.SensorRule != nil {
		sensorRule = *group.Runtime.SensorRule
	}
	watchdog := 0
	if group.Runtime.Watchdog != nil {
		watchdog = *group.Runtime.Watchdog
	}
	status := gm.GroupStatus{
		Group:                group.Runtime.Group,
		Auto:                 auto,
		TimeToAuto:           group.TimeToAuto,
		SensorRule:           sensorRule,
		Error:                group.Error,
		Presence:             group.Presence,
		WindowsOpened:        group.Opened,
		TimeToLeave:          group.PresenceTimeout,
		CorrectionInterval:   correctionInterval,
		SetpointLeds:         group.Setpoint,
		SlopeStartAuto:       slopeStartAuto,
		SlopeStartManual:     slopeStartManual,
		SlopeStopAuto:        slopeStopAuto,
		SlopeStopManual:      slopeStopManual,
		Brightness:           group.Brightness,
		CeilingTemperature:   group.CeilingTemperature,
		CeilingHumidity:      group.CeilingHumidity,
		Temperature:          group.Temperature,
		CO2:                  group.CO2,
		COV:                  group.COV,
		Hygrometry:           group.Hygrometry,
		Leds:                 group.Runtime.Leds,
		Blinds:               group.Runtime.Blinds,
		Sensors:              group.Runtime.Sensors,
		Hvacs:                group.Runtime.Hvacs,
		RuleBrightness:       group.Runtime.RuleBrightness,
		RulePresence:         group.Runtime.RulePresence,
		Watchdog:             watchdog,
		FriendlyName:         name,
		FirstDayOffset:       group.Runtime.FirstDayOffset,
		FirstDay:             group.Runtime.FirstDay,
		SetpointLedsFirstDay: group.FirstDaySetpoint,
	}

	return database.UpdateGroupStatus(s.db, status)
}

func (s *Service) groupRun(group *Group) error {
	ticker := time.NewTicker(time.Second)
	go func() {
		group.Counter = 0
		for {
			select {
			case events := <-group.Event:
				for eventType, e := range events {
					switch eventType {
					case EventStop:
						return

					case EventChange:
						group.updateConfig(e)

					case EventManual:
						rlog.Info("Received manual event ", group)
						watchdog := 0
						if group.Runtime.Watchdog != nil {
							watchdog = *group.Runtime.Watchdog
						}
						group.TimeToAuto = watchdog
						s.setpointLed(group)
						s.dumpGroupStatus(*group)

					case EventBlind:
						rlog.Info("Received blind event ", group)
						s.setpointBlind(group, group.SetpointBlinds, group.SetpointSlatBlinds)
					case EventHvac:
						rlog.Info("Received HVAC event ", group)
						s.setpointHvac(group, group.ShiftTemp)
					}
				}
			case <-ticker.C:
				group.Counter++
				if s.isManualMode(group) {
					if group.Sensors.Count() > 0 {
						// compute TimeToAuto and switch back to Auto mode
						if group.Runtime.Watchdog != nil {
							//decrease only when a rule exists
							if group.TimeToAuto <= 0 {
								auto := true
								group.Runtime.Auto = &auto
								rlog.Info("Switch group " + strconv.Itoa(group.Runtime.Group) + " back to Automatic mode")
							} else {
								group.TimeToAuto--
							}
						}
					} else {
						//When no sensor are presents the group stay in manual mode forever
						auto := false
						group.Runtime.Auto = &auto
					}
				}

				//force to compute presence to be sure that the status is consistent even if the group was is manual mode
				s.computePresence(group)
				s.computeOpen(group)
				s.computeSensorTemperatureAndHumidity(group)
				s.computeBrightness(group)

				//force re-check mode due to the switch back manual to auto mode
				if !s.isManualMode(group) {

					if group.Presence != group.LastPresenceStatus {
						if !group.Presence {
							//leave room empty
							group.Setpoint = 0
							group.FirstDaySetpoint = 0
							rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) + " : is now empty")
						} else {
							//someone come In
							s.updateBrightness(group)
							rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) + " : someone Come in")
						}
						s.setpointLed(group)
					} else {
						if group.Presence {
							if group.Runtime.CorrectionInterval == nil || group.Counter >= *group.Runtime.CorrectionInterval {
								s.updateBrightness(group)
								s.setpointLed(group)
								group.Counter = 0
							}
						} else {
							if group.Runtime.CorrectionInterval == nil || group.Counter >= *group.Runtime.CorrectionInterval {
								group.Setpoint = 0
								group.FirstDaySetpoint = 0
								group.Counter = 0
							}
							s.setpointLed(group)
						}
					}
				} else {
					// Force reset led to be sure that the state is re-apply
					// if one led switch back from manual mode
					s.setpointLed(group)
				}

				err := s.dumpGroupStatus(*group)
				if err != nil {
					rlog.Errorf("Cannot dump status to database for " + strconv.Itoa(group.Runtime.Group) + " err " + err.Error())
				}
			}
		}
	}()
	return nil
}

func (s *Service) isManualMode(group *Group) bool {
	if group.Runtime.Auto == nil || *group.Runtime.Auto == false {
		return true
	}
	return false
}

func (s *Service) isPresenceDetected(group *Group) bool {
	for _, driver := range group.Sensors.Items() {
		sensor, _ := ToSensorEvent(driver)
		_, ok := group.SensorsIssue.Get(sensor.Mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		if sensor.Presence {
			return true
		}
	}
	return false
}

func (s *Service) hasWindowOpened(group *Group) bool {
	for _, driver := range group.Blinds.Items() {
		blind, _ := ToBlindEvent(driver)
		_, ok := group.BlindsIssue.Get(blind.Mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		if blind.WindowStatus1 || blind.WindowStatus2 {
			return true
		}
	}
	return false
}

func (s *Service) computeOpen(group *Group) {
	opened := s.hasWindowOpened(group)

	if group.Blinds.Count() == 0 {
		//force status close when there is no blind in the group
		group.Opened = false
		return
	}
	group.Opened = opened
}

func (s *Service) computePresence(group *Group) {
	group.LastPresenceStatus = group.Presence
	presence := s.isPresenceDetected(group)

	if group.Sensors.Count() == 0 {
		//stay in manual mode
		group.Presence = true
		return
	}

	if group.Runtime.RulePresence != nil {
		if !presence {
			if group.PresenceTimeout <= 0 {
				group.PresenceTimeout = *group.Runtime.RulePresence
			} else {
				group.PresenceTimeout--
				if group.PresenceTimeout == 0 {
					group.Presence = false
				}
			}
		} else {
			group.Presence = true
			group.PresenceTimeout = *group.Runtime.RulePresence
		}
	} else {
		//no delay for leaving the room detection
		group.Presence = presence
	}
}

func (s *Service) updateBrightness(group *Group) {
	if group.Runtime.RuleBrightness != nil {
		readBrightness := *group.Runtime.RuleBrightness
		if group.Brightness > readBrightness {
			//decrease light
			if group.Runtime.FirstDayOffset != nil {
				offset := *group.Runtime.FirstDayOffset
				group.FirstDaySetpoint -= group.Scale
				if (100-group.FirstDaySetpoint) > offset || group.FirstDaySetpoint == 0 {
					group.Setpoint -= group.Scale
				}
			} else {
				group.Setpoint -= group.Scale
			}
		}
		if group.Brightness < readBrightness {
			group.Setpoint += group.Scale
			if group.Runtime.FirstDayOffset != nil {
				offset := *group.Runtime.FirstDayOffset
				if group.Setpoint <= offset && group.Setpoint != 100 {
					group.FirstDaySetpoint = 0
				} else {
					group.FirstDaySetpoint += group.Scale
				}
			}
		}
	} else {
		// we do not have brightness rule. We expect LEDs to be set to 100
		group.Setpoint = 100
		group.FirstDaySetpoint = 100
	}
}

func (s *Service) computeBrightness(group *Group) {
	//compute sensor values
	refMac := ""
	bright := 0
	for _, driver := range group.Sensors.Items() {
		sensor, _ := ToSensorEvent(driver)
		_, ok := group.SensorsIssue.Get(sensor.Mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		refMac = sensor.Mac
		bright = sensor.Brightness
		break
	}
	if refMac == "" {
		//No sensors in this group
		return
	}
	nbValidSensors := group.Sensors.Count() - group.SensorsIssue.Count()
	if nbValidSensors <= 0 {
		//no valid sensor found
		return
	}

	group.Brightness = bright / nbValidSensors

	sensorRule := gm.SensorAverage
	if group.Runtime.SensorRule != nil {
		sensorRule = *group.Runtime.SensorRule
	}

	for mac, driver := range group.Sensors.Items() {
		if mac == refMac {
			continue
		}
		sensor, _ := ToSensorEvent(driver)

		_, ok := group.SensorsIssue.Get(sensor.Mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}

		switch sensorRule {
		case gm.SensorAverage:
			group.Brightness += sensor.Brightness / nbValidSensors
		case gm.SensorMax:
			if group.Brightness < sensor.Brightness {
				group.Brightness = sensor.Brightness
			}
		case gm.SensorMin:
			if group.Brightness > sensor.Brightness {
				group.Brightness = sensor.Brightness
			}
		}
	}
}

func (s *Service) computeSensorTemperatureAndHumidity(group *Group) {
	//compute sensor values
	refMac := ""
	hum := 0
	temp := 0
	for mac, driver := range group.Sensors.Items() {
		sensor, _ := ToSensorEvent(driver)
		_, ok := group.SensorsIssue.Get(mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		refMac = mac
		hum = sensor.Humidity
		temp = sensor.Temperature
		break
	}
	if refMac == "" {
		//No sensors in this group
		return
	}
	nbValidSensors := group.Sensors.Count() - group.SensorsIssue.Count()
	if nbValidSensors <= 0 {
		//no valid sensor found
		return
	}

	group.CeilingTemperature = temp / nbValidSensors
	group.CeilingHumidity = hum / nbValidSensors

	sensorRule := gm.SensorAverage
	if group.Runtime.SensorRule != nil {
		sensorRule = *group.Runtime.SensorRule
	}
	switch sensorRule {
	case gm.SensorMin:
		group.CeilingTemperature = temp
		group.CeilingHumidity = hum
	}

	for mac, d := range group.Sensors.Items() {
		sensor, _ := ToSensorEvent(d)

		_, ok := group.SensorsIssue.Get(sensor.Mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}

		switch sensorRule {
		case gm.SensorAverage:
			if mac == refMac {
				continue
			}
			group.CeilingTemperature += sensor.Temperature / nbValidSensors
			group.CeilingHumidity += sensor.Humidity / nbValidSensors
		case gm.SensorMax:
			if group.CeilingHumidity < sensor.Humidity {
				group.CeilingHumidity = sensor.Humidity
			}
			if group.CeilingTemperature < sensor.Temperature {
				group.CeilingTemperature = sensor.Temperature
			}
		case gm.SensorMin:
			if group.CeilingHumidity > sensor.Humidity {
				group.CeilingHumidity = sensor.Humidity
			}
			if group.CeilingTemperature > sensor.Temperature {
				group.CeilingTemperature = sensor.Temperature
			}
		}
	}
}

func (s *Service) computeNanosenseInfo(group *Group) {
	//compute nanosense values
	refMac := ""
	hum := 0
	temp := 0
	co2 := 0
	cov := 0
	for mac, driver := range group.Nanosenses.Items() {
		nano, _ := ToNanoEvent(driver)
		_, ok := group.NanosensesIssue.Get(mac)
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		refMac = mac
		hum = nano.Hygrometry
		co2 = nano.CO2
		cov = nano.COV
		temp = nano.Temperature
		break
	}
	if refMac == "" {
		//No sensors in this group
		return
	}
	nbValidSensors := group.Nanosenses.Count() - group.NanosensesIssue.Count()
	if nbValidSensors <= 0 {
		//no valid sensor found
		return
	}

	group.Temperature = temp / nbValidSensors
	group.Hygrometry = hum / nbValidSensors
	group.CO2 = co2 / nbValidSensors
	group.COV = cov / nbValidSensors

	sensorRule := gm.SensorAverage
	if group.Runtime.SensorRule != nil {
		sensorRule = *group.Runtime.SensorRule
	}
	switch sensorRule {
	case gm.SensorMin:
		group.Temperature = temp
		group.Hygrometry = hum
		group.CO2 = co2
		group.COV = cov
	}

	for mac, d := range group.Nanosenses.Items() {
		nano, _ := ToNanoEvent(d)

		_, ok := group.SensorsIssue.Get(nano.Label)
		if ok {
			// do not take it to account a nano with an issue
			continue
		}

		switch sensorRule {
		case gm.SensorAverage:
			if mac == refMac {
				continue
			}
			group.Temperature += nano.Temperature / nbValidSensors
			group.Hygrometry += nano.Hygrometry / nbValidSensors
			group.COV += nano.COV / nbValidSensors
			group.CO2 += nano.CO2 / nbValidSensors
		case gm.SensorMax:
			if group.Hygrometry < nano.Hygrometry {
				group.Hygrometry = nano.Hygrometry
			}
			if group.Temperature < nano.Temperature {
				group.Temperature = nano.Temperature
			}
			if group.CO2 < nano.CO2 {
				group.CO2 = nano.CO2
			}
			if group.COV < nano.COV {
				group.COV = nano.COV
			}
		case gm.SensorMin:
			if group.Hygrometry > nano.Hygrometry {
				group.Hygrometry = nano.Hygrometry
			}
			if group.Temperature > nano.Temperature {
				group.Temperature = nano.Temperature
			}
			if group.CO2 > nano.CO2 {
				group.CO2 = nano.CO2
			}
			if group.COV > nano.COV {
				group.COV = nano.COV
			}
		}
	}
}

func (s *Service) setpointLed(group *Group) {
	if group.Setpoint < 0 {
		group.Setpoint = 0
	}
	if group.Setpoint > 100 {
		group.Setpoint = 100
	}
	if group.FirstDaySetpoint < 0 {
		group.FirstDaySetpoint = 0
	}
	if group.FirstDaySetpoint > 100 {
		group.FirstDaySetpoint = 100
	}
	rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) + " =>  leds setpoint: " + strconv.Itoa(group.Setpoint))
	var slopeStart int
	var slopeStop int
	auto := false
	if group.Runtime.Auto != nil {
		auto = *group.Runtime.Auto
	}
	if auto {
		if group.Runtime.SlopeStartAuto != nil {
			slopeStart = *group.Runtime.SlopeStartAuto
		}
		if group.Runtime.SlopeStopAuto != nil {
			slopeStop = *group.Runtime.SlopeStopAuto
		}
	} else {
		if group.Runtime.SlopeStartManual != nil {
			slopeStart = *group.Runtime.SlopeStartManual
		}
		if group.Runtime.SlopeStopManual != nil {
			slopeStop = *group.Runtime.SlopeStopManual
		}
	}

	for _, led := range group.Runtime.Leds {
		_, ok := group.FirstDay.Get(led)
		setpoint := group.Setpoint
		if auto && ok {
			setpoint = group.FirstDaySetpoint
			rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) + " =>  leds FirstDaySetpoint: " + strconv.Itoa(group.FirstDaySetpoint))
		}
		s.sendLedGroupSetpoint(led, setpoint, slopeStart, slopeStop)
	}
}

func (s *Service) setpointBlind(group *Group, blind *int, slat *int) {
	for _, driver := range group.Runtime.Blinds {
		s.sendBlindGroupSetpoint(driver, blind, slat)
	}
}

func (s *Service) setpointHvac(group *Group, shift *int) {
	for _, driver := range group.Runtime.Hvacs {
		s.sendHvacGroupSetpoint(driver, shift)
	}
}

func (s *Service) sendHvacValues(group *Group) {
	// send hygro/Co2/COV/temp from nanosenses
	for _, driver := range group.Runtime.Hvacs {
		s.sendHvacSpaceValues(driver, group.Temperature, group.CO2, group.COV, group.Hygrometry, group.Opened)
	}
}

func (s *Service) createGroup(runtime gm.GroupConfig) {
	if runtime.Auto == nil {
		auto := true
		runtime.Auto = &auto
	}
	group := Group{
		Event:           make(chan map[string]*gm.GroupConfig),
		Runtime:         runtime,
		Scale:           10,
		Sensors:         cmap.New(),
		SensorsIssue:    cmap.New(),
		Blinds:          cmap.New(),
		BlindsIssue:     cmap.New(),
		Nanosenses:      cmap.New(),
		NanosensesIssue: cmap.New(),
		FirstDay:        cmap.New(),
	}
	for _, sensor := range runtime.Sensors {
		group.Sensors.Set(sensor, SensorEvent{})
	}

	for _, blind := range runtime.Blinds {
		group.Blinds.Set(blind, BlindEvent{})
	}

	for _, nano := range runtime.Nanosenses {
		group.Nanosenses.Set(nano, NanoEvent{})
	}

	for _, led := range runtime.FirstDay {
		group.FirstDay.Set(led, true)
	}
	for _, sensor := range runtime.Sensors {
		//to be sure of the state after a creation or a restart
		group.SensorsIssue.Set(sensor, true)
	}
	for _, label := range runtime.Nanosenses {
		//to be sure of the state after a creation or a restart
		group.NanosensesIssue.Set(label, true)
	}
	for _, blind := range runtime.Blinds {
		//to be sure of the state after a creation or a restart
		group.BlindsIssue.Set(blind, true)
	}
	s.groups[runtime.Group] = group
	s.groupRun(&group)
}

func (s *Service) stopGroup(group gm.GroupConfig) {
	event := make(map[string]*gm.GroupConfig)
	event[EventStop] = nil
	s.groups[group.Group].Event <- event
}

func (s *Service) deleteGroup(group gm.GroupConfig) {
	s.stopGroup(group)
	time.Sleep(time.Second)

	gr, _ := s.groups[group.Group]
	if gr.DbID != "" {
		s.db.DeleteRecord(pconst.DbStatus, pconst.TbGroups, gr)
		s.db.DeleteRecord(pconst.DbConfig, pconst.TbGroups, gr)
	}
	delete(s.groups, group.Group)
}

func (s *Service) reloadGroupConfig(groupID int, newconfig gm.GroupConfig) {
	event := make(map[string]*gm.GroupConfig)
	event[EventChange] = &newconfig
	s.groups[groupID].Event <- event
}

func (gr *Group) updateConfig(new *gm.GroupConfig) {
	if new == nil {
		return
	}
	if new.Auto != gr.Runtime.Auto {
		gr.Runtime.Auto = new.Auto
	}
	if new.SlopeStartManual != nil {
		gr.Runtime.SlopeStartManual = new.SlopeStartManual
	}
	if new.SlopeStopManual != nil {
		gr.Runtime.SlopeStopManual = new.SlopeStopManual
	}
	if new.SlopeStartAuto != nil {
		gr.Runtime.SlopeStartAuto = new.SlopeStartAuto
	}
	if new.SlopeStopAuto != nil {
		gr.Runtime.SlopeStopAuto = new.SlopeStopAuto
	}

	if gr.Runtime.Auto != nil && *gr.Runtime.Auto == false && new.SetpointLeds != nil {
		go func() {
			gr.Setpoint = *new.SetpointLeds
			event := make(map[string]*gm.GroupConfig)
			event[EventManual] = nil
			gr.Event <- event
		}()
	}

	if new.SetpointSlatBlinds != nil || new.SetpointBlinds != nil {
		go func() {
			gr.SetpointSlatBlinds = new.SetpointSlatBlinds
			gr.SetpointBlinds = new.SetpointBlinds
			event := make(map[string]*gm.GroupConfig)
			event[EventBlind] = new
			gr.Event <- event
		}()
	}

	if new.SetpointTempOffset != nil {
		go func() {
			gr.ShiftTemp = new.SetpointTempOffset
			event := make(map[string]*gm.GroupConfig)
			event[EventHvac] = new
			gr.Event <- event
		}()
	}

	if new.CorrectionInterval != nil {
		gr.Runtime.CorrectionInterval = new.CorrectionInterval
		if gr.Counter > *gr.Runtime.CorrectionInterval {
			gr.Counter = *gr.Runtime.CorrectionInterval
		}
	}
	if new.SensorRule != nil {
		gr.Runtime.SensorRule = new.SensorRule
	}
	if new.Leds != nil {
		gr.Runtime.Leds = new.Leds
	}
	if new.FirstDay != nil {
		gr.Runtime.FirstDay = new.FirstDay
		seen := make(map[string]bool)
		for _, led := range new.FirstDay {
			_, ok := gr.FirstDay.Get(led)
			if !ok {
				gr.FirstDay.Set(led, true)
			}
			seen[led] = true
		}
		for mac := range gr.FirstDay.Items() {
			_, ok := seen[mac]
			if !ok {
				gr.FirstDay.Remove(mac)
			}
		}

	}
	if new.FirstDayOffset != nil {
		gr.Runtime.FirstDayOffset = new.FirstDayOffset
	}

	if new.Blinds != nil {
		gr.Runtime.Blinds = new.Blinds
		seen := make(map[string]bool)
		for _, blind := range new.Blinds {
			_, ok := gr.Blinds.Get(blind)
			if !ok {
				gr.Blinds.Set(blind, BlindEvent{})
			}
			seen[blind] = true
			// do not take in to consideration until we received valid information from the blind
			gr.BlindsIssue.Set(blind, true)
		}
		for mac := range gr.Blinds.Items() {
			_, ok := seen[mac]
			if !ok {
				gr.Blinds.Remove(mac)
				_, ok := gr.BlindsIssue.Get(mac)
				if ok {
					gr.BlindsIssue.Remove(mac)
				}
			}
		}
	}
	if new.Nanosenses != nil {
		gr.Runtime.Nanosenses = new.Nanosenses
		seen := make(map[string]bool)
		for _, label := range new.Nanosenses {
			_, ok := gr.Nanosenses.Get(label)
			if !ok {
				gr.Nanosenses.Set(label, NanoEvent{})
			}
			seen[label] = true
			// do not take in to consideration until we received valid information from the nanosense
			gr.NanosensesIssue.Set(label, true)
		}
		for label := range gr.Nanosenses.Items() {
			_, ok := seen[label]
			if !ok {
				gr.Nanosenses.Remove(label)
				_, ok := gr.Nanosenses.Get(label)
				if ok {
					gr.NanosensesIssue.Remove(label)
				}
			}
		}
	}
	if new.Hvacs != nil {
		gr.Runtime.Hvacs = new.Hvacs
	}
	if new.Sensors != nil {
		gr.Runtime.Sensors = new.Sensors
		seen := make(map[string]bool)
		for _, sensor := range new.Sensors {
			_, ok := gr.Sensors.Get(sensor)
			if !ok {
				gr.Sensors.Set(sensor, SensorEvent{})
			}
			seen[sensor] = true
			// do not take in to consideration until we received valid information from the sensor
			gr.SensorsIssue.Set(sensor, true)
		}
		for mac := range gr.Sensors.Items() {
			_, ok := seen[mac]
			if !ok {
				gr.Sensors.Remove(mac)
				_, ok := gr.SensorsIssue.Get(mac)
				if ok {
					gr.SensorsIssue.Remove(mac)
				}
			}
		}
	}
	if new.RuleBrightness != nil {
		gr.Runtime.RuleBrightness = new.RuleBrightness
	}
	if new.RulePresence != nil {
		gr.Runtime.RulePresence = new.RulePresence
		if gr.PresenceTimeout > *gr.Runtime.RulePresence {
			//force decrease counter
			gr.PresenceTimeout = *gr.Runtime.RulePresence
		}
	}
	if new.FriendlyName != nil {
		gr.Runtime.FriendlyName = new.FriendlyName
	}
	if new.SensorRule != nil {
		gr.Runtime.SensorRule = new.SensorRule
	}
	if new.Watchdog != nil {
		gr.Runtime.Watchdog = new.Watchdog
		if gr.TimeToAuto > *gr.Runtime.Watchdog {
			//force decrease counter
			gr.TimeToAuto = *gr.Runtime.Watchdog
		}
	}
}

func (s *Service) onGroupCommand(client network.Client, msg network.Message) {
	payload := msg.Payload()
	payloadStr := string(payload)
	rlog.Info("Received BLE cmd" + msg.Topic() + " : " + payloadStr)
	var cmd SwitchCmd
	err := json.Unmarshal(payload, &cmd)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	grID := cmd.Group
	if _, ok := s.groups[grID]; !ok {
		rlog.Info("Group " + strconv.Itoa(grID) + " not running on this switch skip it")
		return
	}
	group := dgroup.GroupConfig{
		Group:              cmd.Group,
		SetpointLeds:       cmd.Leds,
		SetpointSlatBlinds: cmd.Slats,
		SetpointBlinds:     cmd.Blinds,
		SetpointTempOffset: cmd.TempShift,
	}
	if cmd.Leds != nil {
		auto := false
		group.Auto = &auto
	}
	s.reloadGroupConfig(grID, group)
}
