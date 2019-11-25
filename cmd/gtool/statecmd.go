package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/utils/abi"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	stateCommand = cli.Command{
		Name:  "state",
		Usage: "Query Fractal State",
		Flags: []cli.Flag{
			RpcFlag,
			AddressFlag,
			BlockHashFlag,
			TableFlag,
			StorageKeyFlag,
			AbiFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "account",
				Usage:  "Query account info",
				Action: queryAccount,
				Flags: []cli.Flag{
					RpcFlag,
					AddressFlag,
					BlockHashFlag,
				},
			},
			{
				Name:   "storage",
				Usage:  "Query storage info",
				Action: queryStorage,
				Flags: []cli.Flag{
					RpcFlag,
					AddressFlag,
					BlockHashFlag,
					TableFlag,
					StorageKeyFlag,
					AbiFlag,
				},
			},
		},
	}
)

func queryAccount(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	addrString := ctx.GlobalString(AddressFlag.Name)
	addr := common.HexToAddress(addrString)

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}

	var head types.Block
	err = client.Call(&head, "ftl_headBlock")
	if err != nil {
		log.Error("get head block error", "err", err)
		return err
	}
	log.Info("get head block ok", "height", head.Header.Height, "round", head.Header.Round, "hash", head.FullHash())

	var balance hexutil.Big
	err = client.Call(&balance, "ftl_getBalance", addr, head.FullHash())
	if err != nil {
		log.Error("get balance error", "err", err)
		return err
	}
	log.Info("get balance ok", "addr", addr, "balance", balance.ToInt())

	var code hexutil.Bytes
	err = client.Call(&code, "ftl_getCode", addr, head.FullHash())
	if err != nil {
		log.Error("get balance error", "err", err)
		return err
	}
	log.Info("get code ok", "addr", addr, "len", len(code), "code", hexutil.Encode(code))

	var owner common.Address
	err = client.Call(&owner, "ftl_getContractOwner", addr)
	if err != nil {
		log.Error("get balance error", "err", err)
		return err
	}
	log.Info("get owner ok", "addr", addr, "owner", owner)

	return nil
}

func queryStorage(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	addrString := ctx.GlobalString(AddressFlag.Name)
	addr := common.HexToAddress(addrString)
	bhashString := ctx.GlobalString(BlockHashFlag.Name)
	table := ctx.GlobalString(TableFlag.Name)
	skey := ctx.GlobalString(StorageKeyFlag.Name)
	log.Info("skey data", "skey", skey)

	abiFile := ctx.GlobalString(AbiFlag.Name)
	abidef, _ := ioutil.ReadFile(abiFile)

	var skeyData interface{}
	err := json.Unmarshal([]byte(skey), &skeyData)
	if err != nil {
		log.Error("unmarshal skeys failed", "err", err)
	}

	writer := bytes.NewBuffer([]byte{})
	serializer, err := abi.NewAbiSerializer(string(abidef))
	if err != nil {
		log.Error("queryStorage NewAbiSerializer failed", "err", err)
		return err
	}
	keyType := serializer.GetTableKeyType(table)
	err = serializer.Serialize(skeyData, keyType, writer)
	if err != nil {
		log.Error("can not serilize skey", "err", err)
		return err
	}
	skeyHex := hexutil.Encode(writer.Bytes())

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}

	var blockHash common.Hash
	if bhashString == "" {
		var head types.Block
		err = client.Call(&head, "ftl_headBlock")
		if err != nil {
			log.Error("get head block error", "err", err)
			return err
		}
		log.Info("get head block ok", "height", head.Header.Height, "round", head.Header.Round, "hash", head.FullHash())
		blockHash = head.FullHash()
	} else {
		blockHash = common.HexToHash(bhashString)
	}

	var value hexutil.Bytes
	err = client.Call(&value, "ftl_getStorageAt", addr, table, skeyHex, blockHash)
	if err != nil {
		log.Error("get balance error", "err", err)
		return err
	}
	log.Info("get storage ok", "addr", addr, "table", table, "skey", skeyHex, "value", hexutil.Encode(value))

	return nil
}
