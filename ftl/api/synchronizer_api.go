// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

type StatusResult struct {
	Status         string
	FastSyncMode   string
	FastSyncStatus string
}
type SynchronizerAPI struct {
	ftl fractal
}

func NewSynchronizerAPI(ftl fractal) *SynchronizerAPI {
	return &SynchronizerAPI{ftl}
}

func (s *SynchronizerAPI) Status() StatusResult {
	status := s.ftl.Synchronizer().GetSyncStatus()
	fastSyncMode := s.ftl.Synchronizer().GetFastSyncMode()
	fastSyncStatus := s.ftl.Synchronizer().GetFastSyncStatus()
	return StatusResult{
		status.String(), fastSyncMode.String(), fastSyncStatus.String(),
	}
}

// SynchronizerTestAPI provides an API to change config of sync
type SynchronizerTestAPI struct {
	ftl fractal
}

// NewSynchronizerTestAPI creates a new SynchonizerTestAPI.
func NewSynchronizerTestAPI(ftl fractal) *SynchronizerTestAPI {
	return &SynchronizerTestAPI{ftl}
}

//LongTimeOutOfFixPointPreBlock int
//LongTimeOutOfFixPointFinish   int
//LongTimeOutOfFullfillLongList int
//LongTimeOutOfIntevalList      int
//LongTimeOutOfLongList         int
//
//ShortTimeOutOfSyncVeryHigh int
//ShortTimeOutOfShortLists   int

func (s *SynchronizerTestAPI) ChangeLongTimeOutOfFixPointPreBlock(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().TimeOutOfFixPointPreBlock = time
		return "set LongTimeOutOfFixPointPreBlock" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}
func (s *SynchronizerTestAPI) ChangeLongTimeOutOfFixPointFinish(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().LongTimeOutOfFixPointFinish = time
		return "set LongTimeOutOfFixPointFinish" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}
func (s *SynchronizerTestAPI) ChangeLongTimeOutOfFullfillLongList(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().LongTimeOutOfFullfillLongList = time
		return "set LongTimeOutOfFullfillLongList" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}
func (s *SynchronizerTestAPI) ChangeLongTimeOutOfLongList(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().LongTimeOutOfLongList = time
		return "set LongTimeOutOfLongList" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}

func (s *SynchronizerTestAPI) ChangeLongTimeOutOfIntevalList(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().LongTimeOutOfIntevalList = time
		return "set LongTimeOutOfIntevalList" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}

func (s *SynchronizerTestAPI) ChangeShortTimeOutOfSyncVeryHigh(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().ShortTimeOutOfSyncVeryHigh = time
		return "set ShortTimeOutOfSyncVeryHigh" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}
func (s *SynchronizerTestAPI) ChangeShortTimeOutOfShortLists(time int) string {
	if s.ftl.Config().SyncTest {
		s.ftl.Synchronizer().GetConfig().ShortTimeOutOfShortLists = time
		return "set ShortTimeOutOfShortLists" + string(time)
	} else {
		return "not test network,forbidden to change"
	}
}
