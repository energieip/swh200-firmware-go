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
	SetpointBlinds     *int
	SetpointSlatBlinds *int
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
		counter := 0
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
						group.TimeToAuto = *group.Runtime.Watchdog
						s.setpointLed(group)
						s.dumpGroupStatus(*group)

					case EventBlind:
						rlog.Info("Received blind event ", group)
						s.setpointBlind(group, group.SetpointBlinds, group.SetpointSlatBlinds)
					}
				}
			case <-ticker.C:
				counter++

				if len(group.Sensors) > 0 {
					// compute timetoAuto and switch back to Auto mode
					if group.TimeToAuto <= 0 && (group.Runtime.Auto == nil || *group.Runtime.Auto == false) {
						auto := true
						group.Runtime.Auto = &auto
						rlog.Info("Switch group " + strconv.Itoa(group.Runtime.Group) + " back to Automatic mode")
					}
					if group.TimeToAuto > 0 {
						group.TimeToAuto--
					}
				} else {
					//When no sensor are presents the group stay in manual mode forever
					auto := false
					group.Runtime.Auto = &auto
				}
				s.computeSensorsValues(group)

				if group.Runtime.CorrectionInterval == nil || counter == *group.Runtime.CorrectionInterval {
					if group.Runtime.Auto != nil && *group.Runtime.Auto == true {
						rlog.Info("Group " + strconv.Itoa(group.Runtime.Group) +
							" , presence: " + strconv.FormatBool(group.Presence) + " Brightness: " + strconv.Itoa(group.Brightness))
						if group.Presence {
							if group.Runtime.RuleBrightness != nil {
								readBrightness := *group.Runtime.RuleBrightness
								if group.Brightness > readBrightness {
									group.Setpoint -= group.Scale
								}
								if group.Brightness < readBrightness {
									group.Setpoint += group.Scale
								}
							}
						} else {
							//empty room
							group.Setpoint = 0
						}
					}
					s.setpointLed(group)
					err := s.dumpGroupStatus(*group)
					if err != nil {
						rlog.Errorf("Cannot dump status to database for " + strconv.Itoa(group.Runtime.Group) + " err " + err.Error())
					}
					counter = 0
				}
			}
		}
	}()
	return nil
}

func (s *Service) computeSensorsValues(group *Group) {
	//compute sensor values
	refMac := ""
	for mac := range group.Sensors {
		refMac = mac
		break
	}
	if refMac == "" {
		//No sensors in this group
		return
	}
	presence := group.Sensors[refMac].Presence
	group.Brightness = group.Sensors[refMac].Brightness

	for mac, sensor := range group.Sensors {
		if mac == refMac {
			continue
		}
		if sensor.Presence {
			presence = true
		}
		switch *group.Runtime.SensorRule {
		case gm.SensorAverage:
			group.Brightness += sensor.Brightness / len(group.Sensors)
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

	// manage presence group timeout
	if group.Runtime.RulePresence != nil {
		if !presence {
			if group.PresenceTimeout <= 0 {
				group.PresenceTimeout = *group.Runtime.RulePresence
			} else {
				group.PresenceTimeout--
			}
			if group.PresenceTimeout == 0 {
				group.Presence = false
			}
		} else {
			group.Presence = true
		}
	} else {
		group.Presence = true
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
		Event:   make(chan map[string]*gm.GroupConfig),
		Runtime: runtime,
		Scale:   10,
		Sensors: make(map[string]SensorEvent),
	}
	for _, sensor := range runtime.Sensors {
		group.Sensors[sensor] = SensorEvent{}
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
	}
	if new.SensorRule != nil {
		gr.Runtime.SensorRule = new.SensorRule
	}
	if new.Leds != nil {
		gr.Runtime.Leds = new.Leds
	}
	if new.Sensors != nil {
		gr.Runtime.Sensors = new.Sensors
	}
	if new.RuleBrightness != nil {
		gr.Runtime.RuleBrightness = new.RuleBrightness
	}
	if new.RulePresence != nil {
		gr.Runtime.RulePresence = new.RulePresence
	}
	if new.FriendlyName != nil {
		gr.Runtime.FriendlyName = new.FriendlyName
	}
	if new.SensorRule != nil {
		gr.Runtime.SensorRule = new.SensorRule
	}
	if new.Watchdog != nil {
		gr.Runtime.Watchdog = new.Watchdog
	}
}

func (s *Service) onGroupCommand(client network.Client, msg network.Message) {
	payload := msg.Payload()
	payloadStr := string(payload)
	rlog.Info(msg.Topic() + " : " + payloadStr)
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
	rlog.Info("Group " + strconv.Itoa(grID) + " reload it")
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
