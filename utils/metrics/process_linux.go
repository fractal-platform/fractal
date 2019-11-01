package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func CollectProcessStats(stats *ProcessStats) error {
	readCpuMemStats(stats)
	readDiskStats(stats)
	return nil
}

func readCpuMemStats(stats *ProcessStats) error {
	// Open the process stat file
	inf, err := os.Open(fmt.Sprintf("/proc/%d/stat", os.Getpid()))
	if err != nil {
		return err
	}
	defer inf.Close()
	in := bufio.NewReader(inf)

	// Read the only line and split to key and value
	line, err := in.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	parts := strings.Split(line, " ")
	if len(parts) < 25 {
		return errors.New("wrong stat file format")
	}

	stats.CpuUserTime, _ = strconv.ParseInt(strings.TrimSpace(parts[13]), 10, 64)
	stats.CpuSystemTime, _ = strconv.ParseInt(strings.TrimSpace(parts[14]), 10, 64)
	stats.CurrentTime = time.Now().UnixNano()

	stats.VirtualMem, _ = strconv.ParseInt(strings.TrimSpace(parts[22]), 10, 64)
	stats.RssMem, _ = strconv.ParseInt(strings.TrimSpace(parts[23]), 10, 64)

	return nil
}

func readDiskStats(stats *ProcessStats) error {
	// Open the process disk IO counter file
	inf, err := os.Open(fmt.Sprintf("/proc/%d/io", os.Getpid()))
	if err != nil {
		return err
	}
	defer inf.Close()
	in := bufio.NewReader(inf)

	// Iterate over the IO counter, and extract what we need
	for {
		// Read the next line and split to key and value
		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return err
		}

		// Update the counter based on the key
		switch key {
		case "syscr":
			stats.DiskReadCount = value
		case "syscw":
			stats.DiskWriteCount = value
		case "rchar":
			stats.DiskReadBytes = value
		case "wchar":
			stats.DiskWriteBytes = value
		}
	}
	return nil
}
