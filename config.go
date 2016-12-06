package main

func init() {
	// reassign package vars here to customize
	batteries = []string{"BAT0", "BAT1"}
	netInterfaces = []string{"wlp3s0", "enp0s25"}

	// FontAwesome icons
	icons[wifiIcon] = ""
	icons[wiredIcon] = ""
	icons[volumeIcon] = ""
	icons[muteIcon] = ""
	icons[batteryIcon] = ""
}
