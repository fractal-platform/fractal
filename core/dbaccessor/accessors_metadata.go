package dbaccessor

import (
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db DatabaseReader) int {
	var version int

	enc, _ := db.Get(databaseVersionKey)
	rlp.DecodeBytes(enc, &version)

	return version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db DatabaseWriter, version int) {
	enc, _ := rlp.EncodeToBytes(version)
	if err := db.Put(databaseVersionKey, enc); err != nil {
		log.Crit("Failed to store the database version", "err", err)
	}
}

// ReadChainConfig retrieves the chain config.
func ReadChainConfig(db DatabaseReader) ([]byte, error) {
	return db.Get(chainConfigKey)
}

// WriteChainConfig writes the chain config.
func WriteChainConfig(db DatabaseWriter, data []byte) error {
	return db.Put(chainConfigKey, data)
}
