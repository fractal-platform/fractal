package config

type SyncConfig struct {
	PeriodSyncCycle               int   // Time interval to force syncs, even if few peers are available
	MinRegularPeerCount           int   // Amount of peers desired to start syncing
	MinFastSyncPeerCount          int   //amount for fast sync
	CommonPrefixCount             int   // honest peer amount should be equal or bigger than CommonPrefixCount
	HeightDiff                    int32 // bigger than heightDiff,it  will start fast sync:(HeightDiff >CheckMainChainPostBlockLength+)
	ShortHashListLength           int   //short hash list length
	Interval                      int   //first time to sync long hashList interval
	CheckMainChainPostBlockLength int   //should be less than shortHashListLength

	TimeOutOfFixPointPreBlock     int
	LongTimeOutOfFixPointFinish   int
	LongTimeOutOfFullfillLongList int
	LongTimeOutOfIntevalList      int
	LongTimeOutOfLongList         int

	ShortTimeOutOfSyncVeryHigh int
	ShortTimeOutOfShortLists   int
	ShortTimeOutOfPeriodSync   int
}

// DefaultPoolConfig contains the default configurations for the transaction
// pool.
var DefaultSyncConfig = SyncConfig{
	PeriodSyncCycle:               10,
	MinRegularPeerCount:           1,
	MinFastSyncPeerCount:          1,
	CommonPrefixCount:             3,
	HeightDiff:                    30,
	ShortHashListLength:           30,
	Interval:                      200, //first time to sync long hashList interval
	CheckMainChainPostBlockLength: 20,  //should be less than shortHashListLength

	TimeOutOfFixPointPreBlock:     120000000,
	LongTimeOutOfFixPointFinish:   120000000,
	LongTimeOutOfFullfillLongList: 120000000,
	LongTimeOutOfIntevalList:      120000000,
	LongTimeOutOfLongList:         120000000,

	ShortTimeOutOfSyncVeryHigh: 60000000,
	ShortTimeOutOfShortLists:   60000000,
	ShortTimeOutOfPeriodSync:   60000000,
}

// Sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *SyncConfig) Sanitize() SyncConfig {
	conf := *config
	//if conf.Rejournal < time.Second {
	//	log.Warn("Sanitizing invalid pool journal time", "provided", conf.Rejournal, "updated", time.Second)
	//	conf.Rejournal = time.Second
	//}
	//if conf.PriceLimit < 1 {
	//	log.Warn("Sanitizing invalid pool price limit", "provided", conf.PriceLimit, "updated", DefaultPoolConfig.PriceLimit)
	//	conf.PriceLimit = DefaultPoolConfig.PriceLimit
	//}
	return conf
}
