package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/energieip/common-components-go/pkg/dblind"
	"github.com/energieip/swh200-firmware-go/internal/database"

	"github.com/energieip/common-components-go/pkg/dgroup"
	gm "github.com/energieip/common-components-go/pkg/dgroup"
	"github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

const (
	EventChange = "change"
	EventStop   = "stop"
	EventManual = "manual"
	EventBlind  = "blind"
)

// Group logical
type Group struct {
	Event              chan map[string]*gm.GroupConfig
	Runtime            gm.GroupConfig
	Setpoint           int
	Brightness         int
	Presence           bool
	Slope              int
	TimeToAuto         int
	Scale              int    //brightness correction scale
	DbID               string //Database entry ID
	PresenceTimeout    int
	Error              int
	Sensors            map[string]SensorEvent
	SensorsIssue       map[string]bool
	SetpointBlinds     *int
	SetpointSlatBlinds *int
	LastPresenceStatus bool
	Counter            int
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

	group.Sensors[sensor.Mac] = sensor
	_, ok = group.SensorsIssue[sensor.Mac]
	if ok {
		//sensor no longer problematic
		delete(group.SensorsIssue, sensor.Mac)
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

	group.SensorsIssue[sensor.Mac] = true
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
		Group:              group.Runtime.Group,
		Auto:               auto,
		TimeToAuto:         group.TimeToAuto,
		SensorRule:         sensorRule,
		Error:              group.Error,
		Presence:           group.Presence,
		TimeToLeave:        group.PresenceTimeout,
		CorrectionInterval: correctionInterval,
		SetpointLeds:       group.Setpoint,
		SlopeStartAuto:     slopeStartAuto,
		SlopeStartManual:   slopeStartManual,
		SlopeStopAuto:      slopeStopAuto,
		SlopeStopManual:    slopeStopManual,
		Leds:               group.Runtime.Leds,
		Blinds:             group.Runtime.Blinds,
		Sensors:            group.Runtime.Sensors,
		RuleBrightness:     group.Runtime.RuleBrightness,
		RulePresence:       group.Runtime.RulePresence,
		Watchdog:           watchdog,
		FriendlyName:       name,
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
					}
				}
			case <-ticker.C:
				group.Counter++
				if s.isManualMode(group) {
					if len(group.Sensors) > 0 {
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

				//force re-check mode due to the switch back manual to auto mode
				if !s.isManualMode(group) {
					s.computePresence(group)
					s.computeBrightness(group)
					if group.Presence != group.LastPresenceStatus {
						if !group.Presence {
							//leave room empty
							group.Setpoint = 0
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
	for _, sensor := range group.Sensors {
		_, ok := group.SensorsIssue[sensor.Mac]
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

func (s *Service) computePresence(group *Group) {
	group.LastPresenceStatus = group.Presence
	presence := s.isPresenceDetected(group)

	if len(group.Sensors) == 0 {
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
			group.Setpoint -= group.Scale
		}
		if group.Brightness < readBrightness {
			group.Setpoint += group.Scale
		}
	} else {
		// we do not have brightness rule. We expect LEDs to be set to 100
		group.Setpoint = 100
	}
}

func (s *Service) computeBrightness(group *Group) {
	//compute sensor values
	refMac := ""
	for mac := range group.Sensors {
		_, ok := group.SensorsIssue[mac]
		if ok {
			// do not take it to account a sensor with an issue
			continue
		}
		refMac = mac
		break
	}
	if refMac == "" {
		//No sensors in this group
		return
	}
	nbValidSensors := len(group.Sensors) - len(group.SensorsIssue)
	if nbValidSensors <= 0 {
		//no valid sensor found
		return
	}

	group.Brightness = group.Sensors[refMac].Brightness / nbValidSensors

	sensorRule := gm.SensorAverage
	if group.Runtime.SensorRule != nil {
		sensorRule = *group.Runtime.SensorRule
	}

	for mac, sensor := range group.Sensors {
		if mac == refMac {
			continue
		}

		_, ok := group.SensorsIssue[sensor.Mac]
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

func (s *Service) setpointLed(group *Group) {
	if group.Setpoint < 0 {
		group.Setpoint = 0
	}
	if group.Setpoint > 100 {
		group.Setpoint = 100
	}
	rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) + " =>  leds setpoint: " + strconv.Itoa(group.Setpoint))
	var slopeStart int
	var slopeStop int
	if group.Runtime.Auto != nil && *group.Runtime.Auto == true {
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
		s.sendLedGroupSetpoint(led, group.Setpoint, slopeStart, slopeStop)
	}
}

func (s *Service) setpointBlind(group *Group, blind *int, slat *int) {
	for _, driver := range group.Runtime.Blinds {
		s.sendBlindGroupSetpoint(driver, blind, slat)
	}
}

func (s *Service) createGroup(runtime gm.GroupConfig) {
	if runtime.Auto == nil {
		auto := true
		runtime.Auto = &auto
	}
	group := Group{
		Event:        make(chan map[string]*gm.GroupConfig),
		Runtime:      runtime,
		Scale:        10,
		Sensors:      make(map[string]SensorEvent),
		SensorsIssue: make(map[string]bool),
	}
	for _, sensor := range runtime.Sensors {
		group.Sensors[sensor] = SensorEvent{}
	}
	for _, sensor := range runtime.Sensors {
		//to be sure of the state after a creation or a restart
		group.SensorsIssue[sensor] = true
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
		s.db.DeleteRecord(gm.DbStatusName, gm.TableStatusName, gr)
		s.db.DeleteRecord(dblind.DbConfig, gm.TableStatusName, gr)
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
	if new.Blinds != nil {
		gr.Runtime.Blinds = new.Blinds
	}
	if new.Sensors != nil {
		gr.Runtime.Sensors = new.Sensors
		seen := make(map[string]bool)
		for _, sensor := range new.Sensors {
			_, ok := gr.Sensors[sensor]
			if !ok {
				gr.Sensors[sensor] = SensorEvent{}
			}
			seen[sensor] = true
			// do not take in to consideration until we received valid information from the sensor
			gr.SensorsIssue[sensor] = true
		}
		for mac := range gr.Sensors {
			_, ok := seen[mac]
			if !ok {
				delete(gr.Sensors, mac)
				_, ok := gr.SensorsIssue[mac]
				if ok {
					delete(gr.SensorsIssue, mac)
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
	}
	if cmd.Leds != nil {
		auto := false
		group.Auto = &auto
	}
	s.reloadGroupConfig(grID, group)
}
