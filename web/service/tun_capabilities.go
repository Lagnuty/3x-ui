package service

import (
	"strconv"
	"strings"
)

func linuxCapabilityFlags(processStatus string) (netAdmin, netRaw bool) {
	for _, line := range strings.Split(processStatus, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "CapEff:" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			return false, false
		}
		return value&(uint64(1)<<12) != 0, value&(uint64(1)<<13) != 0
	}
	return false, false
}
