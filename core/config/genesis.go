// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rlp"
)

var DefaultGenesisRound = uint64(time.Date(2019, 12, 1, 0, 0, 0, 0, time.UTC).Unix())

// GenesisMismatchError raised when stored genesis block conflicts with the input genesis config
type GenesisMismatchError struct {
	StoredGenesisHash common.Hash
	NewGenesisHash    common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("db already contains an conflict genesis block (have %x, new %x)", e.StoredGenesisHash[:8], e.NewGenesisHash[:8])
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount state.AccountForStorage

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// Genesis specifies the header fields of genesis block, and the genesis state of the chain.
type Genesis struct {
	Round      uint64         `json:"round"            gencodec:"required"`
	PubKey     []byte         `json:"pubKey"           gencodec:"required"`
	Sig        []byte         `json:"sig"              gencodec:"required"`
	Coinbase   common.Address `json:"miner"            gencodec:"required"`
	Difficulty *big.Int       `json:"difficulty"       gencodec:"required"`
	Alloc      GenesisAlloc   `json:"alloc"            gencodec:"required"`
}

// DefaultGenesisBlock returns the main net genesis block.
func DefaultMainnetGenesisBlock() *Genesis {
	return &Genesis{
		Round:      DefaultGenesisRound,
		PubKey:     []byte{},
		Sig:        []byte{},
		Difficulty: big.NewInt(16),
		//Alloc:      decodePreAlloc(mainnetAllocData),
	}
}

// DefaultTestnetGenesisBlock returns the test network genesis block.
func DefaultTestnetGenesisBlock() *Genesis {
	return &Genesis{
		Round:      uint64(time.Date(2019, 10, 31, 0, 0, 0, 0, time.UTC).Unix() * params.RoundsPerSecond),
		PubKey:     []byte{},
		Sig:        []byte{},
		Difficulty: new(big.Int).Mul(big.NewInt(1e17), big.NewInt(150)),
		Alloc:      decodePreAlloc(testnetAllocData),
	}
}

// DefaultTestnet2GenesisBlock returns the test2 network genesis block.
func DefaultTestnet2GenesisBlock() *Genesis {
	return &Genesis{
		Round:      uint64(time.Date(2019, 10, 31, 0, 0, 0, 0, time.UTC).Unix() * params.RoundsPerSecond),
		PubKey:     []byte{},
		Sig:        []byte{},
		Difficulty: new(big.Int).Mul(big.NewInt(1e17), big.NewInt(150)),
		Alloc:      decodePreAlloc(testnet2AllocData),
	}
}

// DefaultTestnet3GenesisBlock returns the test3 network genesis block.
func DefaultTestnet3GenesisBlock() *Genesis {
	return &Genesis{
		Round:      uint64(time.Date(2019, 10, 31, 0, 0, 0, 0, time.UTC).Unix() * params.RoundsPerSecond),
		PubKey:     []byte{},
		Sig:        []byte{},
		Difficulty: new(big.Int).Mul(big.NewInt(1e17), big.NewInt(150)),
		Alloc:      decodePreAlloc(testnet3AllocData),
	}
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (g *Genesis) ToBlock(db dbwrapper.Database) *types.Block {
	if db == nil {
		db = dbwrapper.NewMemDatabase()
	}
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		//log.Info("genesis","addr",addr,"balance",account.Balance)
		code, _ := hexutil.Decode(account.Code)
		statedb.SetCode(addr, code)
		statedb.SetContractOwner(addr, account.Owner)
		//statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	root := statedb.IntermediateRoot(false)
	block := &types.Block{
		Header: types.BlockHeader{
			ParentHash: common.Hash{},
			Round:      g.Round,
			Sig:        g.Sig,
			Coinbase:   g.Coinbase,
			Difficulty: g.Difficulty,
			Height:     0,
			Amount:     1,
			GasLimit:   params.GenesisGasLimit,
			GasUsed:    0,
			StateHash:  root,
			TxHash:     common.Hash{},
			Confirms:   []common.Hash{},
			FullSig:    []byte{},
		},
		Body: types.BlockBody{
			Transactions: []*types.Transaction{},
		},
	}
	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	return block
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db dbwrapper.Database) (*types.Block, error) {
	block := g.ToBlock(db)

	dbaccessor.WriteBlock(db, block)
	dbaccessor.WriteBlockChilds(db, block.FullHash(), []common.Hash{})
	dbaccessor.WriteHashList(db, block.Header.Round, []*types.BlockRoundHash{{
		Round:      block.Header.Round,
		SimpleHash: block.SimpleHash(),
		FullHash:   block.FullHash(),
	}})
	dbaccessor.WriteBloom(db, block.FullHash(), &types.Bloom{})
	dbaccessor.WriteHeadBlockHash(db, block.FullHash())
	dbaccessor.WriteGenesisBlockHash(db, block.FullHash())
	dbaccessor.WriteBlockStateCheck(db, block.FullHash(), types.BlockStateChecked)
	dbaccessor.WriteHeightBlocks(db, block.Header.Height, []common.Hash{block.FullHash()})
	return block, nil
}

// SetupChainConfig determine the genesis block of the current chain, and write the genesis block to db.
func SetupGenesisBlock(db dbwrapper.Database, genesis *Genesis) (common.Hash, error) {
	stored := dbaccessor.ReadGenesisBlockHash(db)
	if (stored != common.Hash{}) {
		if genesis == nil {
			// input genesis config is nil, so the stored genesis hash is returned
			return stored, nil
		} else {
			hash := genesis.ToBlock(nil).FullHash()
			if hash != stored {
				return hash, &GenesisMismatchError{stored, hash}
			} else {
				return hash, nil
			}
		}
	} else {
		if genesis != nil {
			// use input genesis config to generate genesis block, and write to db
			block, err := genesis.Commit(db)
			return block.FullHash(), err
		} else {
			// use default mainnet config
			block, err := DefaultMainnetGenesisBlock().Commit(db)
			return block.FullHash(), err
		}
	}
}

type balanceAllocStruct struct {
	Addr    *big.Int
	Balance *big.Int
	Code    string
	Owner   common.Address
	Storage state.Storage
}

func decodePreAlloc(data string) GenesisAlloc {
	var p []balanceAllocStruct
	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := make(GenesisAlloc, len(p))
	for _, account := range p {
		ga[common.BigToAddress(account.Addr)] = GenesisAccount{
			Balance: account.Balance,
			Code:    account.Code,
			Owner:   account.Owner,
			Storage: account.Storage,
		}
	}
	return ga
}
