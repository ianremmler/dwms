package main

import "strings"

func init() {
	batteries = []string{"BAT0", "BAT1"}
	format = func(s []string) string { return strings.Join(s, " | ") + " " } // add a space next to systray
}
