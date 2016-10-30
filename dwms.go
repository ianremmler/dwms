package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
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

var (
	itemSeparator = " | "
	items         = []int{batteryItem, timeItem}

	batteryPath      = "/sys/class/power_supply/BAT%d/"
	batterySymbols   = map[string]string{"Charging": "+", "Discharging": "-", "Full": "="}
	batterySeparator = "/"
	batteries        = []int{0}

	timeLayout = "2006-01-02 15:04"
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

func batteryStatus(bat int) string {
	pct, err := sysfsIntVal(fmt.Sprintf(batteryPath+"capacity", bat))
	if err != nil {
		return "?"
	}
	status, err := sysfsStringVal(fmt.Sprintf(batteryPath+"status", bat))
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("%d%s", pct, batterySymbols[status])
}

func updateStatus(x *xgb.Conn, root xproto.Window) {
	var statuses []string
	for _, item := range items {
		switch item {
		case batteryItem:
			var batStats []string
			for _, bat := range batteries {
				batStats = append(batStats, batteryStatus(bat))
			}
			statuses = append(statuses, strings.Join(batStats, batterySeparator))
		case timeItem:
			statuses = append(statuses, time.Now().Format(timeLayout))
		}
	}
	status := strings.Join(statuses, itemSeparator)

	xproto.ChangeProperty(x, xproto.PropModeReplace, root, xproto.AtomWmName,
		xproto.AtomString, byte(8), uint32(len(status)), []byte(status))
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
