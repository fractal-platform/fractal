// +build !linux

package metrics

import "errors"

func CollectProcessStats(stats *ProcessStats) error {
	return errors.New("Not implemented")
}
