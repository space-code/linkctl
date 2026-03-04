package simulator

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type simctlDevice struct {
	UDID        string `json:"udid"`
	Name        string `json:"name"`
	State       string `json:"state"`
	IsAvailable bool   `json:"isAvailable"`
}

type simctlOutput struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

func CheckToolsAvailable() map[string]bool {
	available := make(map[string]bool)
	for _, tool := range []string{"xcrun"} {
		_, err := exec.LookPath(tool)
		available[tool] = err == nil
	}
	return available
}

func ListIOSDevices() ([]string, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "booted", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl failed: %w", err)
	}

	var payload simctlOutput
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse simctl JSON: %w", err)
	}

	var names []string
	for _, devices := range payload.Devices {
		for _, d := range devices {
			if strings.EqualFold(d.State, "Booted") && d.IsAvailable {
				names = append(names, fmt.Sprintf("%s (%s)", d.Name, d.UDID))
			}
		}
	}
	return names, nil
}

func GetBootedIOSDevices() ([]simctlDevice, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "booted", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl failed: %w", err)
	}

	var payload simctlOutput
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse simctl JSON: %w", err)
	}

	var devices []simctlDevice
	for _, devices := range payload.Devices {
		for _, d := range devices {
			if strings.EqualFold(d.State, "Booted") && d.IsAvailable {
				devices = append(devices, d)
			}
		}
	}
	return devices, nil
}
