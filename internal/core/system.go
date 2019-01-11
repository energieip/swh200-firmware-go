package core

import (
	"os/exec"
	"strings"

	"github.com/romana/rlog"
)

//GetLastSystemUpgradeDate return the date of the last system upgrade
func GetLastSystemUpgradeDate() string {
	cmd := exec.Command("tail", "-n", "1", "/var/log/apt/term.log")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	values := strings.SplitN(line, ":", 2)
	if len(values) < 2 {
		return ""
	}
	return strings.TrimSpace(values[1])
}

//SystemUpgrade check and update system
func SystemUpgrade() {
	rlog.Info("Check for system Update")

	cmd := exec.Command("apt-get", "update")
	_, err := cmd.CombinedOutput()
	if err != nil {
		rlog.Error("apt-get update finished with " + err.Error())
		return
	}

	cmd = exec.Command("apt-get", "upgrade", "-y")
	output, err := cmd.CombinedOutput()
	if err != nil {
		rlog.Info("Apt-get dist-upgrade finished with " + err.Error())
		return
	}
	rlog.Info("Upgrade " + string(output))

	cmd = exec.Command("apt-get", "dist-upgrade", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Info("Apt-get dist-upgrade finished with " + err.Error())
		return
	}
	rlog.Info("Dist-Upgrade " + string(output))

	cmd = exec.Command("apt-get", "autoremove", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Info("Apt-get autoremove finished with " + err.Error())
		return
	}
	rlog.Info("Autoremove " + string(output))

	cmd = exec.Command("apt-get", "autoclean", "-y")
	output, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Info("Apt-get autoremove finished with " + err.Error())
		return
	}
	rlog.Info("Autoclean " + string(output))

	rlog.Warn("Ask for a system reboot????")
}
