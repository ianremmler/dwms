// dwms is a dwm status generator.
//
// Assign custom values to exported identifiers in config.go to configure.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

type statusFunc func() string

const (
	battSysPath = "/sys/class/power_supply"
	netSysPath  = "/sys/class/net"
)

var (
	ssidRE    = regexp.MustCompile(`SSID:\s+(.*)`)
	bitrateRE = regexp.MustCompile(`tx bitrate:\s+(\d+)`)
	signalRE  = regexp.MustCompile(`signal:\s+(-\d+)`)
	amixerRE  = regexp.MustCompile(`\[(\d+)%]\s*\[(\w+)]`)
	xconn     *xgb.Conn
	xroot     xproto.Window
)

var WifiFmt = func(dev, ssid string, bitrate, signal int, up bool) string {
	if !up {
		return ""
	}
	return fmt.Sprintf("ω%s/%d/%d", ssid, bitrate, signal)
}

var WiredFmt = func(dev string, speed int, up bool) string {
	if !up {
		return ""
	}
	return "ε" + strconv.Itoa(speed)
}

var NetFmt = func(devs []string) string {
	return strings.Join(filterEmpty(devs), " ")
}

var BatteryDevFmt = func(pct int, state string) string {
	return strconv.Itoa(pct) + map[string]string{"Charging": "+", "Discharging": "-"}[state]
}

var BatteryFmt = func(bats []string) string {
	return "β" + strings.Join(bats, "/")
}

var AudioFmt = func(vol int, muted bool) string {
	return map[bool]string{false: "ν", true: "μ"}[muted] + strconv.Itoa(vol)
}

var TimeFmt = func(t time.Time) string {
	return t.Format("τ01/02-15:04")
}

var StatusFmt = func(stats []string) string {
	return " " + strings.Join(filterEmpty(stats), " ") + " "
}

func wifiStatus(dev string) (string, int, int) {
	ssid, bitrate, signal := "", 0, 0
	out, err := exec.Command("iw", "dev", dev, "link").Output()
	if err != nil {
		return ssid, bitrate, signal
	}
	if match := ssidRE.FindSubmatch(out); len(match) >= 2 {
		ssid = string(match[1])
	}
	if match := bitrateRE.FindSubmatch(out); len(match) >= 2 {
		if br, err := strconv.Atoi(string(match[1])); err == nil {
			bitrate = br
		}
	}
	if match := signalRE.FindSubmatch(out); len(match) >= 2 {
		if sig, err := strconv.Atoi(string(match[1])); err == nil {
			signal = sig
		}
	}
	return ssid, bitrate, signal
}

func wiredStatus(dev string) int {
	speed, err := sysfsIntVal(filepath.Join(netSysPath, dev, "speed"))
	if err != nil {
		return 0
	}
	return speed
}

func netDevStatus(dev string) string {
	status, err := sysfsStringVal(filepath.Join(netSysPath, dev, "operstate"))
	up := err == nil && status == "up"
	if _, err = os.Stat(filepath.Join(netSysPath, dev, "wireless")); err == nil {
		ssid, bitrate, signal := wifiStatus(dev)
		return WifiFmt(dev, ssid, bitrate, signal, up)
	}
	speed := wiredStatus(dev)
	return WiredFmt(dev, speed, up)
}

func netStatus(devs ...string) statusFunc {
	return func() string {
		var netStats []string
		for _, dev := range devs {
			netStats = append(netStats, netDevStatus(dev))
		}
		return NetFmt(netStats)
	}
}

func batteryDevStatus(batt string) string {
	pct, err := sysfsIntVal(filepath.Join(battSysPath, batt, "capacity"))
	if err != nil {
		return Unknown
	}
	status, err := sysfsStringVal(filepath.Join(battSysPath, batt, "status"))
	if err != nil {
		return Unknown
	}
	return BatteryDevFmt(pct, status)
}

func batteryStatus(batts ...string) statusFunc {
	return func() string {
		var battStats []string
		for _, batt := range batts {
			battStats = append(battStats, batteryDevStatus(batt))
		}
		return BatteryFmt(battStats)
	}
}

func audioStatus(args ...string) statusFunc {
	args = append(args, []string{"get", "Master"}...)
	return func() string {
		out, err := exec.Command("amixer", args...).Output()
		if err != nil {
			return Unknown
		}
		match := amixerRE.FindSubmatch(out)
		if len(match) < 3 {
			return Unknown
		}
		vol, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return Unknown
		}
		muted := (string(match[2]) == "off")
		return AudioFmt(vol, muted)
	}
}

func timeStatus() string {
	return TimeFmt(time.Now())
}

func status() string {
	var stats []string
	for _, item := range Items {
		stats = append(stats, item())
	}
	return StatusFmt(stats)
}

func setStatus(statusText string) {
	xproto.ChangeProperty(xconn, xproto.PropModeReplace, xroot, xproto.AtomWmName,
		xproto.AtomString, 8, uint32(len(statusText)), []byte(statusText))
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

func filterEmpty(strings []string) []string {
	filtStrings := strings[:0]
	for _, str := range strings {
		if str != "" {
			filtStrings = append(filtStrings, str)
		}
	}
	return filtStrings
}

func run() {
	setStatus(status())
	defer setStatus("") // cleanup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	update := time.Tick(UpdatePeriod)
	for {
		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGUSR1:
				setStatus(status())
			default:
				return
			}
		case <-update:
			setStatus(status())
		}
	}
}

func main() {
	var err error
	xconn, err = xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}
	defer xconn.Close()
	xroot = xproto.Setup(xconn).DefaultScreen(xconn).Root
	run()
}
