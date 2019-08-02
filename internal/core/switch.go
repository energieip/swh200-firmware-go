package core

import (
	"encoding/json"
	"strconv"

	"github.com/energieip/common-components-go/pkg/network"
	"github.com/romana/rlog"
)

type SwitchCmd struct {
	Group     int   `json:"group"`
	Leds      *int  `json:"leds,omitempty"`
	Slats     *int  `json:"slats,omitempty"`
	Blinds    *int  `json:"blinds,omitempty"`
	TempShift *int  `json:"heat,omitempty"`     //temperature shift in 1/10°C
	Action    *bool `json:"action,omitempty"`   //true/false   (press/release)
	ButtonA   *bool `json:"button_A,omitempty"` //true/false  (0/1)
	ButtonB   *bool `json:"button_B,omitempty"` //true/false  (0/1)
}

func (s *Service) onSwitchCmd(client network.Client, msg network.Message) {
	payload := msg.Payload()
	payloadStr := string(payload)
	rlog.Info(msg.Topic() + " : " + payloadStr)
	var cmd SwitchCmd
	err := json.Unmarshal(payload, &cmd)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	url := "/write/group/" + strconv.Itoa(cmd.Group) + "/commands"

	s.clusterSendCommand(url, payloadStr)
	s.localSendCommand(url, payloadStr)
}
