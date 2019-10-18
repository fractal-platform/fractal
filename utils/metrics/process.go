package metrics

import (
	"github.com/rcrowley/go-metrics"
	"time"
)

type ProcessStats struct {
	CpuUserTime   int64 // Cpu use time(/proc/[pid]/stat utime)
	CpuSystemTime int64 // Cpu use time(/proc/[pid]/stat stime)
	CurrentTime   int64 // Current time

	VirtualMem int64 // Virtual memory size
	RssMem     int64 // Resident memory size

	DiskReadCount  int64 // Number of read operations executed
	DiskReadBytes  int64 // Total number of bytes read
	DiskWriteCount int64 // Number of write operations executed
	DiskWriteBytes int64 // Total number of byte written
}

func CollectProcessMetrics() {
	if metrics.UseNilMetrics {
		return
	}

	cpuUsageGauge := metrics.GetOrRegisterGaugeFloat64("system/cpu/usage", nil)
	vmGauge := metrics.GetOrRegisterGauge("system/mem/vm", nil)
	rssGauge := metrics.GetOrRegisterGauge("system/mem/rss", nil)
	diskReads := metrics.GetOrRegisterMeter("system/disk/readcount", nil)
	diskReadBytes := metrics.GetOrRegisterMeter("system/disk/readdata", nil)
	diskWrites := metrics.GetOrRegisterMeter("system/disk/writecount", nil)
	diskWriteBytes := metrics.GetOrRegisterMeter("system/disk/writedata", nil)

	// Create collectors
	stats := make([]*ProcessStats, 2)
	for i := 0; i < len(stats); i++ {
		stats[i] = new(ProcessStats)
	}

	// Iterate loading the different stats and updating the meters
	for i := 1; ; i++ {
		location1 := i % 2
		location2 := (i - 1) % 2

		err := CollectProcessStats(stats[location1])
		if err != nil {
			return
		}

		if i > 1 {
			cpuUse := (stats[location1].CpuUserTime + stats[location1].CpuSystemTime) -
				(stats[location2].CpuUserTime + stats[location2].CpuSystemTime)
			cpuUse = cpuUse * 1e7
			cpuUsage := 100.0 * float64(cpuUse) / float64(stats[location1].CurrentTime - stats[location2].CurrentTime)
			cpuUsageGauge.Update(cpuUsage)
		} else {
			cpuUsageGauge.Update(0)
		}

		vmGauge.Update(int64(stats[location1].VirtualMem))
		rssGauge.Update(int64(stats[location1].RssMem))

		diskReads.Mark(stats[location1].DiskReadCount - stats[location2].DiskReadCount)
		diskReadBytes.Mark(stats[location1].DiskReadBytes - stats[location2].DiskReadBytes)
		diskWrites.Mark(stats[location1].DiskWriteCount - stats[location2].DiskWriteCount)
		diskWriteBytes.Mark(stats[location1].DiskWriteBytes - stats[location2].DiskWriteBytes)

		time.Sleep(10 * time.Second)
	}
}
