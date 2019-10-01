package core

import (
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/romana/rlog"

	sd "github.com/energieip/common-components-go/pkg/dswitch"
	"github.com/energieip/swh200-firmware-go/internal/database"
)

var (
	Configuration = "/etc/dhcpcd.conf"
	Reference     = Configuration + ".ref"
	Temp          = "/tmp/dhcpcd.conf"
)

func copyFile(src string, dst string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) updateIPConfig(ip string, elt sd.SwitchDefinition) {
	// IP == 0 means dhcp, else fix IP address
	cfg := database.GetSwitchConfig(s.db)
	if cfg.IP == "" {
		cfg.IP = "0" // DHCP by default
	}
	if ip == cfg.IP {
		database.UpdateSwitchConfig(s.db, elt)
		rlog.Info("Correct IP nothing to change")
		return
	}
	if ip == "0" {
		copyFile(Reference, Configuration)
		database.UpdateSwitchConfig(s.db, elt)
		rlog.Info("Restart Switch to switch in DHCP mode")
		time.Sleep(5 * time.Second)
		cmd := exec.Command("reboot")
		_, err := cmd.CombinedOutput()
		if err != nil {
			rlog.Error("Reboot finished with " + err.Error())
		}
		return
	}

	rlog.Info("Change IP configuration to " + ip)
	if _, err := os.Stat(Temp); !os.IsNotExist(err) {
		os.Remove(Temp)
	}

	copyFile(Reference, Temp)

	f, err := os.OpenFile(Temp, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	dump := "interface eth0\nstatic ip_address=" + ip + "/24\nstatic routers=192.168.0.2\nstatic domain_name_servers=192.168.0.2\n"

	if _, err = f.WriteString(dump); err != nil {
		f.Close()
		return
	}
	f.Close()
	time.Sleep(1 * time.Second)
	copyFile(Temp, Configuration)
	database.UpdateSwitchConfig(s.db, elt)
	time.Sleep(5 * time.Second)

	rlog.Info("Restart Switch")
	cmd := exec.Command("reboot")
	_, err = cmd.CombinedOutput()
	if err != nil {
		rlog.Error("Reboot finished with " + err.Error())
	}
}
