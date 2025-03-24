package dra

import "strings"

// GetPciAddressPrefix returns the prefix of a PCI address
// [domain]:[bus]:[device].[function] -> [domain]:[bus]
// e.g. 0000:af:00.1 -> 0000:af
func GetPciAddressPrefix(pciAddress string) string {
	parts := strings.Split(pciAddress, ":")
	if len(parts) == 3 {
		return parts[0] + ":" + parts[1]
	}
	return ""
}
