package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	unknown      = "?"
	updatePeriod = 5 * time.Second
)

var (
	chargeStatus  = map[string]string{"Charging": "+", "Discharging": "-"}
	items         = []func() string{netStatus, batteryStatus, audioStatus, timeStatus}
	batteries     = []string{"BAT0", "BAT1"}
	netInterfaces = []string{"wlp3s0", "enp0s25"}
)

func wifiFmt(dev, ssid string, bitrate, signal int, isUp bool) string {
	if !isUp {
		return ""
	}
	return fmt.Sprintf("ω%s/%d/%d", ssid, bitrate, signal)
}

func wiredFmt(dev string, speed int, isUp bool) string {
	if !isUp {
		return ""
	}
	return fmt.Sprintf("ε%d", speed)
}

func netFmt(devs []string) string {
	upDevs := devs[:0]
	for _, dev := range devs {
		if len(dev) > 0 {
			upDevs = append(upDevs, dev)
		}
	}
	return strings.Join(upDevs, " ")
}

func batteryDevFmt(pct int, status string) string {
	return fmt.Sprintf("%d%s", pct, chargeStatus[status])
}

func batteryFmt(bats []string) string {
	return "β" + strings.Join(bats, "/")
}

func audioFmt(vol int, isMuted bool) string {
	sym := "ν"
	if isMuted {
		sym = "μ"
	}
	return sym + fmt.Sprintf("%d", vol)
}

func timeFmt(t time.Time) string {
	return t.Format("τ01/02-15:04")
}

func statusFmt(stats []string) string {
	okStats := stats[:0]
	for _, stat := range stats {
		if len(stat) > 0 {
			okStats = append(okStats, stat)
		}
	}
	return " " + strings.Join(okStats, " ") + " "
}
