package rpi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// flexBool tolerates different formats of the boolean field in lsblk output —
// older versions return "0"/"1" as text, newer true/false.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	switch s {
	case "true", "1":
		*b = true
	case "false", "0", "null", "":
		*b = false
	default:
		return fmt.Errorf("unexpected boolean value in lsblk output: %q", s)
	}
	return nil
}

// flexInt64 tolerates numbers written as text in lsblk output.
type flexInt64 int64

func (n *flexInt64) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("unexpected numeric value in lsblk output: %q", s)
	}
	*n = flexInt64(v)
	return nil
}

// lsblkDevice represents a single device from `lsblk -J` output.
type lsblkDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Size        flexInt64     `json:"size"`
	Type        string        `json:"type"`
	RM          flexBool      `json:"rm"`
	Model       string        `json:"model"`
	Mountpoints []*string     `json:"mountpoints"`
	Children    []lsblkDevice `json:"children"`
}

// lsblkOutput represents the main JSON object from `lsblk -J`.
type lsblkOutput struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

// runLsblk runs lsblk with the given arguments and parses the JSON result.
func runLsblk(args ...string) (*lsblkOutput, error) {
	out, err := runLogged(nil, "lsblk", args...)
	if err != nil {
		return nil, fmt.Errorf("cannot read block devices list: %w", err)
	}
	var parsed lsblkOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("cannot parse lsblk output: %w", err)
	}
	return &parsed, nil
}

// ListRemovableDrives parses `lsblk -J -b -o NAME,SIZE,TYPE,RM,MODEL` output
// and returns EXCLUSIVELY removable media (rm==true and type=="disk").
// Internal drives will never be on the list.
func ListRemovableDrives() ([]Drive, error) {
	parsed, err := runLsblk("-J", "-b", "-o", "NAME,SIZE,TYPE,RM,MODEL")
	if err != nil {
		return nil, err
	}
	var drives []Drive
	for _, dev := range parsed.Blockdevices {
		if !bool(dev.RM) || dev.Type != "disk" {
			continue // skip anything that is not a removable disk
		}
		drives = append(drives, Drive{
			Device: "/dev/" + dev.Name,
			SizeH:  HumanSize(int64(dev.Size)),
			Model:  strings.TrimSpace(dev.Model),
		})
	}
	return drives, nil
}
