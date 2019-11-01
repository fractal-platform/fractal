// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package sync contains the implementation of fractal sync strategy.
package sync

// SyncStatus is the top status for sync process
type SyncStatus int

const (
	SyncStatusBegin SyncStatus = iota
	SyncStatusInit
	SyncStatusFastSync
	SyncStatusNormal
	SyncStatusPeerSync
	SyncStatusEnd
)

func (s SyncStatus) String() string {
	if s <= SyncStatusBegin || s >= SyncStatusEnd {
		return "Unknown"
	}

	list := [...]string{
		"Init",
		"FastSync",
		"Normal",
		"PeerSync"}
	return list[s-1]
}

// FastSyncMode is the mode for fast sync
type FastSyncMode int

const (
	FastSyncModeBegin = iota
	FastSyncModeNone
	FastSyncModeEasy
	FastSyncModeComplex
	FastSyncModeEnd
)

func (m FastSyncMode) String() string {
	if m <= FastSyncModeBegin || m >= FastSyncModeEnd {
		return "Unknown"
	}

	list := [...]string{
		"None",
		"Easy",
		"Complex"}
	return list[m-1]
}

// FastSyncStatus is the child status for StatusFastSync
type FastSyncStatus int

const (
	FastSyncStatusBegin FastSyncStatus = iota

	FastSyncStatusNone
	FastSyncStatusShortHashList

	// for complex mode
	FastSyncStatusLongHashList
	FastSyncStatusCheckMainChain

	// for fix point fetch
	FastSyncStatusFixPointHashTree
	FastSyncStatusFixPointPreBlocks
	FastSyncStatusFixPointPreStates
	FastSyncStatusFixPointPostBlocks
	FastSyncStatusFixPointBestBlocks

	FastSyncStatusEnd
)

func (s FastSyncStatus) String() string {
	if s <= FastSyncStatusBegin || s >= FastSyncStatusEnd {
		return "Unknown"
	}

	list := [...]string{
		"None",
		"ShortHashList",
		"LongHashList",
		"CheckMainChain",
		"FixPointHashTree",
		"FixPointPreBlocks",
		"FixPointPreStates",
		"FixPointPostBlocks",
		"FixPointBestBlocks",
	}
	return list[s-1]
}

func (s *Synchronizer) GetSyncStatus() SyncStatus {
	return s.status.Load().(SyncStatus)
}

func (s *Synchronizer) IsSyncStatusNormal() bool {
	status := s.GetSyncStatus()
	return status == SyncStatusNormal
}

func (s *Synchronizer) IsSyncStatusInit() bool {
	status := s.GetSyncStatus()
	return status == SyncStatusInit
}

func (s *Synchronizer) GetFastSyncMode() FastSyncMode {
	return s.fastSyncMode.Load().(FastSyncMode)
}

func (s *Synchronizer) GetFastSyncStatus() FastSyncStatus {
	return s.fastSyncStatus.Load().(FastSyncStatus)
}

func (s *Synchronizer) changeSyncStatus(status SyncStatus) {
	old := s.status.Load()
	if old == nil {
		s.log.Info("change sync status", "from", old, "to", status)
	} else {
		s.log.Info("change sync status", "from", old.(SyncStatus), "to", status)
	}
	s.status.Store(status)
}

func (s *Synchronizer) changeFastSyncMode(mode FastSyncMode) {
	old := s.fastSyncMode.Load()
	if old == nil {
		s.log.Info("change fast sync mode", "from", old, "to", mode)
	} else {
		s.log.Info("change fast sync mode", "from", old.(FastSyncMode), "to", mode)
	}
	s.fastSyncMode.Store(mode)
}

func (s *Synchronizer) changeFastSyncStatus(status FastSyncStatus) {
	old := s.fastSyncStatus.Load()
	if old == nil {
		s.log.Info("change fast sync status", "from", old, "to", status)
	} else {
		s.log.Info("change fast sync status", "from", old.(FastSyncStatus), "to", status)
	}
	s.fastSyncStatus.Store(status)
}
