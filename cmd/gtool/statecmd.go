package main

import (
	"fmt"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/rpc/client"
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
			TableFlag,
			StorageKeyFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "account",
				Usage:  "Query account info",
				Action: queryAccount,
				Flags: []cli.Flag{
					RpcFlag,
					AddressFlag,
				},
			},
			{
				Name:   "storage",
				Usage:  "Query storage info",
				Action: queryStorage,
				Flags: []cli.Flag{
					RpcFlag,
					AddressFlag,
					TableFlag,
					StorageKeyFlag,
				},
			},
			{
				Name:   "whitelist",
				Usage:  "Query transfer white list",
				Action: queryTransferWhiteList,
				Flags: []cli.Flag{
					RpcFlag,
				},
			},
			{
				Name:   "blacklist",
				Usage:  "Query transfer black list",
				Action: queryTransferBlackList,
				Flags: []cli.Flag{
					RpcFlag,
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
	table := ctx.GlobalString(TableFlag.Name)
	skey := ctx.GlobalString(StorageKeyFlag.Name)

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

	var value hexutil.Bytes
	err = client.Call(&value, "ftl_getStorageAt", addr, table, skey, head.FullHash())
	if err != nil {
		log.Error("get balance error", "err", err)
		return err
	}
	log.Info("get storage ok", "addr", addr, "table", table, "value", hexutil.Encode(value))

	return nil
}

func queryTransferWhiteList(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}

	var whiteList string
	err = client.Call(&whiteList, "ftl_getTransferWhiteList", "latest")
	if err != nil {
		log.Error("get white list error", "err", err)
		return err
	}

	if whiteList == "" {
		fmt.Printf("The transfer white list is empty\n")
	} else {
		fmt.Printf("The transfer white list is: \n%s\n", whiteList)
	}

	return nil
}

func queryTransferBlackList(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}

	var blackList string
	err = client.Call(&blackList, "ftl_getTransferBlackList", "latest")
	if err != nil {
		log.Error("get black list error", "err", err)
		return err
	}

	if blackList == "" {
		fmt.Printf("The transfer black list is empty\n")
	} else {
		fmt.Printf("The transfer black list is: \n%s\n", blackList)
	}

	return nil
}
