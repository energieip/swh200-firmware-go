package core

import (
	"encoding/json"
	"strconv"

	"github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

type SwitchCmd struct {
	Group  int  `json:"group"`
	Leds   *int `json:"leds,omitempty"`
	Slats  *int `json:"slats,omitempty"`
	Blinds *int `json:"blinds,omitempty"`
}

func (s *Service) onSwitchCmd(client network.Client, msg network.Message) {
	payload := msg.Payload()
	payloadStr := string(payload)
	rlog.Info("Switch: Received topic: " + msg.Topic() + " payload: " + payloadStr)
	var cmd SwitchCmd
	err := json.Unmarshal(payload, &cmd)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	url := "/read/group/" + strconv.Itoa(cmd.Group) + "/commands"

	err = s.clusterSendCommand(url, payloadStr)
	if err != nil {
		rlog.Errorf("Cannot send command to Group " + strconv.Itoa(cmd.Group) + " err: " + err.Error())
	} else {
		rlog.Debug("Command to Group has been sent to " + strconv.Itoa(cmd.Group) + " on topic: " + url + " dump: " + payloadStr)
	}

	err = s.localSendCommand(url, payloadStr)
	if err != nil {
		rlog.Errorf("Cannot send command to Group on local broker" + strconv.Itoa(cmd.Group) + " err: " + err.Error())
	} else {
		rlog.Debug("sCommand to Group has been sent on local broker to " + strconv.Itoa(cmd.Group) + " on topic: " + url + " dump: " + payloadStr)
	}
}
