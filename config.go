package main

import (
	"strings"
	"time"
)

func init() {
	// reassign package vars here to customize

	icons[wifiIcon] = "ω"
	icons[wiredIcon] = "ε"
	icons[volumeIcon] = "α"
	icons[batteryIcon] = "β"
	icons[timeIcon] = "τ"

	updatePeriod = 1 * time.Second
	batteries = []string{"BAT0", "BAT1"}
	netInterfaces = []string{"wlp3s0", "enp0s25"}
	timeFormat = func(t time.Time) string {
		return icons[timeIcon] + t.Format("01/02-15:04")
	}
	statusFormat = func(s []string) string {
		return " " + strings.Join(s, " ") + " "
	}
}
