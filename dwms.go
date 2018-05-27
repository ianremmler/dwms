package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

const (
	batSysPath = "/sys/class/power_supply"
	netSysPath = "/sys/class/net"
)

var (
	ssidRE    = regexp.MustCompile(`SSID:\s+(.*)`)
	bitrateRE = regexp.MustCompile(`tx bitrate:\s+(\d+)`)
	signalRE  = regexp.MustCompile(`signal:\s+(-\d+)`)
	amixerRE  = regexp.MustCompile(`\[(\d+)%]\s*\[(\w+)]`)
	xconn     *xgb.Conn
	xroot     xproto.Window
)

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
	isUp := err == nil && status == "up"

	_, err = os.Stat(filepath.Join(netSysPath, dev, "wireless"))
	isWifi := !os.IsNotExist(err)

	if isWifi {
		ssid, bitrate, signal := wifiStatus(dev)
		return wifiFmt(dev, ssid, bitrate, signal, isUp)
	}
	speed := wiredStatus(dev)
	return wiredFmt(dev, speed, isUp)
}

func netStatus() string {
	var netStats []string
	for _, dev := range netInterfaces {
		netStats = append(netStats, netDevStatus(dev))
	}
	return netFmt(netStats)
}

func batteryDevStatus(bat string) string {
	pct, err := sysfsIntVal(filepath.Join(batSysPath, bat, "capacity"))
	if err != nil {
		return unknown
	}
	status, err := sysfsStringVal(filepath.Join(batSysPath, bat, "status"))
	if err != nil {
		return unknown
	}
	return batteryDevFmt(pct, status)
}

func batteryStatus() string {
	var batStats []string
	for _, bat := range batteries {
		batStats = append(batStats, batteryDevStatus(bat))
	}
	return batteryFmt(batStats)
}

func audioStatus() string {
	out, err := exec.Command("amixer", "get", "Master").Output()
	if err != nil {
		return unknown
	}
	match := amixerRE.FindSubmatch(out)
	if len(match) < 3 {
		return unknown
	}
	vol, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return unknown
	}
	isMuted := (string(match[2]) == "off")
	return audioFmt(vol, isMuted)
}

func timeStatus() string {
	return timeFmt(time.Now())
}

func status() string {
	var stats []string
	for _, item := range items {
		stats = append(stats, item())
	}
	return statusFmt(stats)
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

func main() {
	var err error
	xconn, err = xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}
	defer xconn.Close()
	xroot = xproto.Setup(xconn).DefaultScreen(xconn).Root
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	update := time.Tick(updatePeriod)

	setStatus(status())
loop:
	for {
		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGUSR1:
				setStatus(status())
			default:
				break loop
			}
		case <-update:
			setStatus(status())
		}
	}
	setStatus("") // cleanup
}
