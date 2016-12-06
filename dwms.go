package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

type itemID int

const (
	batteryItem itemID = iota
	timeItem
	audioItem
	netItem
)

const (
	batSysPath = "/sys/class/power_supply"
	netSysPath = "/sys/class/net"
)

type iconID int

const (
	noIcon iconID = iota
	volumeIcon
	muteIcon
	timeIcon
	wifiIcon
	wiredIcon
	batteryIcon
	chargeIcon
	dischargeIcon
	fullIcon
	unknownIcon
)

var icons = map[iconID]string{
	noIcon:        "",
	volumeIcon:    "v:",
	muteIcon:      "m:",
	timeIcon:      "",
	wiredIcon:     "e:",
	wifiIcon:      "w:",
	batteryIcon:   "b:",
	chargeIcon:    "+",
	dischargeIcon: "-",
	fullIcon:      "=",
	unknownIcon:   "?",
}

var (
	updatePeriod = 5 * time.Second
	items        = []itemID{netItem, batteryItem, audioItem, timeItem}
	statusFormat = statusFmt

	netInterfaces = []string{"wlan0", "eth0"}
	wifiFormat    = wifiFmt
	wiredFormat   = wiredFmt
	netFormat     = netFmt

	batteries    = []string{"BAT0"}
	batteryIcons = map[string]iconID{
		"Charging": chargeIcon, "Discharging": dischargeIcon, "Full": fullIcon,
	}
	batteryDevFormat = batteryDevFmt
	batteryFormat    = batteryFmt

	audioFormat = audioFmt

	timeFormat = timeFmt
)

func wifiStatus(dev string, isUp bool) (string, bool) {
	ssid, bitrate, signal := "", 0, 0

	cmd := exec.Command("iw", "dev", dev, "link")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", false
	}
	scan := bufio.NewScanner(out)
	if cmd.Start() != nil {
		return "", false
	}
	for scan.Scan() {
		key, value := "", ""
		kv := strings.SplitN(scan.Text(), ": ", 2)
		if len(kv) == 2 {
			key, value = strings.TrimSpace(kv[0]), kv[1]
		}
		switch key {
		case "SSID":
			ssid = value
		case "tx bitrate":
			value = strings.SplitN(value+" ", " ", 2)[0]
			if br, err := strconv.ParseFloat(value, 64); err == nil {
				bitrate = int(br)
			}
		case "signal":
			value = strings.SplitN(value+" ", " ", 2)[0]
			if sig, err := strconv.Atoi(value); err == nil {
				signal = sig
			}
		}
	}
	cmd.Wait()

	return wifiFormat(dev, ssid, bitrate, signal, isUp)
}

func wiredStatus(dev string, isUp bool) (string, bool) {
	speed := 0
	if spd, err := sysfsIntVal(filepath.Join(netSysPath, dev, "speed")); err == nil {
		speed = spd
	}
	return wiredFormat(dev, speed, isUp)
}

func netDevStatus(dev string) (string, bool) {
	status, err := sysfsStringVal(filepath.Join(netSysPath, dev, "operstate"))
	isUp := true
	if err != nil || status != "up" {
		isUp = false
	}

	_, err = os.Stat(filepath.Join(netSysPath, dev, "wireless"))
	isWifi := !os.IsNotExist(err)

	if isWifi {
		return wifiStatus(dev, isUp)
	}
	return wiredStatus(dev, isUp)
}

func netStatus() string {
	var netStats []string
	for _, dev := range netInterfaces {
		devStat, ok := netDevStatus(dev)
		if ok {
			netStats = append(netStats, devStat)
		}
	}
	return netFormat(netStats)
}

func wifiFmt(dev, ssid string, bitrate, signal int, isUp bool) (string, bool) {
	return fmt.Sprintf("%s%s/%d/%d", icons[wifiIcon], ssid, bitrate, signal), isUp
}
func wiredFmt(dev string, speed int, isUp bool) (string, bool) {
	return fmt.Sprintf("%s%d", icons[wiredIcon], speed), isUp
}

func netFmt(devs []string) string {
	return strings.Join(devs, " ")
}

func batteryDevStatus(bat string) string {
	pct, err := sysfsIntVal(filepath.Join(batSysPath, bat, "capacity"))
	if err != nil {
		return icons[unknownIcon]
	}
	status, err := sysfsStringVal(filepath.Join(batSysPath, bat, "status"))
	if err != nil {
		return icons[unknownIcon]
	}
	return batteryDevFormat(pct, status)
}

func batteryStatus() string {
	var batStats []string
	for _, bat := range batteries {
		batStats = append(batStats, batteryDevStatus(bat))
	}
	return batteryFormat(batStats)
}

func batteryDevFmt(pct int, status string) string {
	return fmt.Sprintf("%d%s", pct, icons[batteryIcons[status]])
}

func batteryFmt(bats []string) string {
	return icons[batteryIcon] + strings.Join(bats, "/")
}

func audioStatus() string {
	volStr, err := exec.Command("ponymix", "get-volume").Output()
	if err != nil {
		return icons[unknownIcon]
	}
	vol, err := strconv.Atoi(string(bytes.TrimSpace(volStr)))
	if err != nil {
		return icons[unknownIcon]
	}
	isMuted := (exec.Command("ponymix", "is-muted").Run() == nil)
	return audioFormat(vol, isMuted)
}

func audioFmt(vol int, isMuted bool) string {
	icon := volumeIcon
	if isMuted {
		icon = muteIcon
	}
	return fmt.Sprintf("%s%d", icons[icon], vol)
}

func timeStatus() string {
	return timeFormat(time.Now())
}

func timeFmt(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

func statusFmt(s []string) string {
	return " " + strings.Join(s, " | ") + " "
}

func updateStatus(x *xgb.Conn, root xproto.Window) {
	var stats []string
	for _, item := range items {
		switch item {
		case batteryItem:
			stats = append(stats, batteryStatus())
		case audioItem:
			stats = append(stats, audioStatus())
		case netItem:
			stats = append(stats, netStatus())
		case timeItem:
			stats = append(stats, timeStatus())
		}
	}
	status := statusFormat(stats)

	xproto.ChangeProperty(x, xproto.PropModeReplace, root, xproto.AtomWmName,
		xproto.AtomString, 8, uint32(len(status)), []byte(status))
}

func sysfsIntVal(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return 0, err
	}
	return val, nil
}

func sysfsStringVal(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(data)), nil
}

func main() {
	x, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	root := xproto.Setup(x).DefaultScreen(x).Root
	for t := time.Tick(updatePeriod); ; <-t {
		updateStatus(x, root)
	}
}
