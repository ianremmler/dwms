package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

const (
	batteryItem = iota
	timeItem
)

const (
	batteryPath = "/sys/class/power_supply"
)

var (
	items  = []int{batteryItem, timeItem}
	format = func(s []string) string { return strings.Join(s, " | ") }

	batteries      = []string{"BAT0"}
	batterySymbols = map[string]string{"Charging": "+", "Discharging": "-", "Full": "="}
	batteryFormat  = func(s []string) string { return strings.Join(s, "/") }

	timeFormat = func(t time.Time) string { return t.Format("2006-01-02 15:04") }
)

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

func batteryStatus(bat string) string {
	pct, err := sysfsIntVal(filepath.Join(batteryPath, bat, "capacity"))
	if err != nil {
		return "?"
	}
	status, err := sysfsStringVal(filepath.Join(batteryPath, bat, "status"))
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%d%s", pct, batterySymbols[status])
}

func updateStatus(x *xgb.Conn, root xproto.Window) {
	var stats []string
	for _, item := range items {
		switch item {
		case batteryItem:
			var batStats []string
			for _, bat := range batteries {
				batStats = append(batStats, batteryStatus(bat))
			}
			stats = append(stats, batteryFormat(batStats))
		case timeItem:
			stats = append(stats, timeFormat(time.Now()))
		}
	}
	status := format(stats)

	xproto.ChangeProperty(x, xproto.PropModeReplace, root, xproto.AtomWmName,
		xproto.AtomString, 8, uint32(len(status)), []byte(status))
}

func main() {
	x, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	root := xproto.Setup(x).DefaultScreen(x).Root
	tick := time.Tick(10 * time.Second)
	updateStatus(x, root)
	for range tick {
		updateStatus(x, root)
	}
}
