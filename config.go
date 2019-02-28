package main

import (
	"time"
)

const (
	Unknown      = "?"
	UpdatePeriod = 5 * time.Second
)

var Items = []statusFunc{
	netStatus("wlp3s0", "enp0s25"),
	batteryStatus("BAT0", "BAT1"),
	audioStatus("-M"),
	timeStatus,
}
