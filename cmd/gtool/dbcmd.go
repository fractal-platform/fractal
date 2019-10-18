package main

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	dbCommand = cli.Command{
		Name:  "database",
		Usage: "Query Fractal Database",
		Flags: []cli.Flag{
			StateRootHashFlag,
			DbPathFlag,
			OutPutPathFlag,
			Keccak256HashFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "dump",
				Usage:  "dump state json",
				Action: dumpStateJson,
				Flags: []cli.Flag{
					StateRootHashFlag,
					DbPathFlag,
					OutPutPathFlag,
				},
			},
			{
				Name:   "size",
				Usage:  "get storage size of (block, transaction, package...)",
				Action: getSize,
				Flags: []cli.Flag{
					DbPathFlag,
					Keccak256HashFlag,
				},
			},
		},
	}
)

func dumpStateJson(ctx *cli.Context) error {
	initLogger(ctx)

	stateRootHash := ctx.GlobalString(StateRootHashFlag.Name)
	dbPath := ctx.GlobalString(DbPathFlag.Name)
	outputPath := ctx.GlobalString(OutPutPathFlag.Name)
	if outputPath == "" {
		outputPath = "state.json"
	}

	db, err := dbwrapper.NewLDBDatabase(dbPath, 768, 2048)
	if err != nil {
		log.Error("init level db failed", "dbpath", dbPath, "err", err)
		return err
	}

	sdb := state.NewDatabase(db)
	stateDb, err := state.New(common.HexToHash(stateRootHash), sdb)
	if err != nil {
		log.Error("init state failed", "dbpath", dbPath, "hash", stateRootHash, "err", err)
		return err
	}
	stateDb.DumpAllState(outputPath)
	log.Info("Dump Ok")
	return nil
}

func getSize(ctx *cli.Context) error {
	initLogger(ctx)

	dbPath := ctx.GlobalString(DbPathFlag.Name)
	hashStr := ctx.GlobalString(Keccak256HashFlag.Name)

	db, err := dbwrapper.NewLDBDatabase(dbPath, 768, 2048)
	if err != nil {
		log.Error("init level db failed", "dbpath", dbPath, "err", err)
		return err
	}
	hash := common.HexToHash(hashStr)

	// block
	headData := dbaccessor.ReadBlockHeaderRLP(db, hash)
	if len(headData) != 0 {
		log.Info("This is a block hash")

		bodyData := dbaccessor.ReadBlockBodyRLP(db, hash)
		receiptData := dbaccessor.ReadReceiptsRLP(db, hash)
		receipts := dbaccessor.ReadReceipts(db, hash)
		bloomData := dbaccessor.ReadBloomRLP(db, hash)

		log.Info("Block size", "headSize", len(headData), "bodySize", len(bodyData), "receiptNum", len(receipts), "receiptSize", len(receiptData), "bloomSize", len(bloomData), "total", len(headData)+len(bodyData)+len(receiptData)+len(bloomData))
		return nil
	}

	// transaction
	txData, _ := dbaccessor.ReadTxLookupEntryRLP(db, hash)
	if len(txData) != 0 {
		log.Info("This is a transaction hash")
		log.Info("TxLookup size", "txLookupSize", len(txData))
		return nil
	}

	// tx package
	pkgData, _ := dbaccessor.ReadTxPkgRLP(db, hash)
	if len(pkgData) != 0 {
		log.Info("This is a package hash")
		log.Info("package size", "pkgSize", len(pkgData))
		return nil
	}

	log.Info("The hash does not match any blocks, transactions or packages.")
	return nil
}
