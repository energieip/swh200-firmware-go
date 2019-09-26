package core

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/romana/rlog"
)

func (s *Service) activateGPIOs() {
	rlog.Info("Activate KSZ Switch")
	cmd := exec.Command("gpio", "write", "44", "1")
	_, err := cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 44 1 update finished with " + err.Error())
		return
	}

	rlog.Info("Activate PSE 1 Switch")
	cmd = exec.Command("gpio", "write", "7", "1")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 7 1 update finished with " + err.Error())
		return
	}
	time.Sleep(20 * time.Second)

	rlog.Info("Activate PSE 2 Switch")
	cmd = exec.Command("gpio", "write", "1", "1")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 1 1 update finished with " + err.Error())
		return
	}
}

func (s *Service) resetPSE() {
	rlog.Info("Down PSE 1 Switch")
	cmd := exec.Command("gpio", "write", "7", "0")
	_, err := cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 7 1 update finished with " + err.Error())
		return
	}

	rlog.Info("Down PSE 2 Switch")
	cmd = exec.Command("gpio", "write", "1", "0")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 1 1 update finished with " + err.Error())
		return
	}

	rlog.Info("Activate PSE 1 Switch")
	cmd = exec.Command("gpio", "write", "7", "1")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 7 1 update finished with " + err.Error())
		return
	}
	time.Sleep(20 * time.Second)

	rlog.Info("Activate PSE 2 Switch")
	cmd = exec.Command("gpio", "write", "1", "1")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio write 1 1 update finished with " + err.Error())
		return
	}
}

func (s *Service) getGPIOStates() map[int]int {
	status := make(map[int]int)

	//BAES
	baes := s.getGPIOState(43)
	res := 1
	if baes == 1 {
		res = 0
	}
	status[0] = res
	//Puls 1
	status[1] = s.getGPIOState(17)
	//Puls 2
	status[2] = s.getGPIOState(16)
	//Puls 3
	status[3] = s.getGPIOState(13)
	//Puls 4
	status[4] = s.getGPIOState(12)
	//Puls 5+
	status[5] = s.getGPIOState(11)
	return status
}

func (s *Service) getGPIOState(gpio int) int {
	cmd := exec.Command("gpio", "read", strconv.Itoa(gpio))
	res, err := cmd.CombinedOutput()
	if err != nil {
		rlog.Error("gpio read " + strconv.Itoa(gpio) + " finished with " + err.Error())
		return 0
	}
	result := strings.Trim(string(res), "\r")
	result = strings.Trim(result, "\n")

	val, err := strconv.Atoi(result)
	if err != nil {
		rlog.Error("Conversion issue " + err.Error())
		return 0
	}
	return val
}
