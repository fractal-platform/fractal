package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"path"
	"strconv"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/abi"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
)

// gtool packer --rpc http://127.0.0.1:8545 --id 0 start

var (
	packerCommand = cli.Command{
		Name:  "packer",
		Usage: "Manage Fractal Packer",
		Flags: []cli.Flag{
			RpcFlag,
			PackerIdFlag,
			ChainIdFlag,
			KeyFolderFlag,
			PasswordFlag,
			AbiFlag,
			PackerRpcAddressFlag,
			PackerCoinbaseFlag,
			PackerPubKeyFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "start",
				Usage:  "Start pack service",
				Action: startPacking,
				Flags: []cli.Flag{
					RpcFlag,
					PackerIdFlag,
				},
			},
			{
				Name:   "stop",
				Usage:  "Stop pack service",
				Action: stopPacking,
				Flags: []cli.Flag{
					RpcFlag,
				},
			},
			{
				Name:   "setPacker",
				Usage:  "Call Contract",
				Action: setPacker,
				Flags: []cli.Flag{
					RpcFlag,
					ChainIdFlag,
					KeyFolderFlag,
					PasswordFlag,
					AbiFlag,
					PackerIdFlag,
					PackerRpcAddressFlag,
					PackerCoinbaseFlag,
					PackerPubKeyFlag,
				},
			},
		},
	}
)

// For packer
func startPacking(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	packerId := uint32(ctx.GlobalUint64(PackerIdFlag.Name))

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}
	err = client.Call(nil, "admin_startPacking", packerId)
	if err != nil {
		log.Error("start packing error", "err", err)
		return err
	}

	return nil
}

func stopPacking(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)

	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}
	client.Call(nil, "admin_stopPacking")

	return nil
}

// For administrator
func setPacker(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)

	abiFile := ctx.GlobalString(AbiFlag.Name)
	abidef, _ := ioutil.ReadFile(abiFile)

	packerId := uint32(ctx.GlobalUint64(PackerIdFlag.Name))
	packerRpcAddressString := ctx.GlobalString(PackerRpcAddressFlag.Name)
	packerCoinbaseString := ctx.GlobalString(PackerCoinbaseFlag.Name)
	packerPubKeyString := ctx.GlobalString(PackerPubKeyFlag.Name)

	buffer := bytes.NewBufferString("[")
	buffer.WriteString(strconv.Itoa(int(packerId)) + ",")
	buffer.WriteString("\"" + packerRpcAddressString + "\",")
	buffer.WriteString("\"" + packerCoinbaseString[:] + "\",")

	// packerPubKey
	packerPubKey, _ := hexutil.Decode(packerPubKeyString)
	buffer.WriteString("[")
	for i := 0; i < len(packerPubKey)-1; i++ {
		buffer.WriteString(strconv.Itoa(int(packerPubKey[i])))
		buffer.WriteString(",")
	}
	buffer.WriteString(strconv.Itoa(int(packerPubKey[len(packerPubKey)-1])))
	buffer.WriteString("]]")
	argsString := buffer.String()
	log.Info("generate args string ok", "argsString", argsString)

	actionUint, err := utils.String2Uint64("setkey")
	if err != nil {
		log.Error("parse action failed", "err", err)
		return err
	}
	actionBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(actionBytes, actionUint) // use LittleEndian

	var argsData interface{}
	err = json.Unmarshal([]byte(argsString), &argsData)
	if err != nil {
		log.Error("unmarshal args failed", "err", err)
		return err
	}

	writer := bytes.NewBuffer(actionBytes)
	serializer, err := abi.NewAbiSerializer(string(abidef))
	if err != nil {
		log.Error("setPacker NewAbiSerializer failed", "err", err)
		return err
	}
	err = serializer.Serialize(argsData, "setkey", writer)
	if err != nil {
		log.Error("serialize args failed", "err", err)
		return err
	}
	actionSlice := writer.Bytes()
	log.Info("generate action bytes ok", "actionSlice", hexutil.Encode(actionSlice))

	toAddr := common.HexToAddress(params.PackerKeyContractAddr)

	//signer
	chainid := ctx.GlobalInt(ChainIdFlag.Name)
	signer := types.NewEIP155Signer(uint64(chainid))

	//key
	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)
	accountKeyFile := path.Join(folder, "account.json")
	accountKey, err := keys.LoadAccountKey(accountKeyFile, password)
	if err != nil {
		log.Error("load account key error", "err", err)
		return err
	}

	var nonce uint64
	var hexNonce hexutil.Uint64
	client, err := rpcclient.Dial(rpc)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpc)
		return err
	}
	err = client.Call(&hexNonce, "txpool_getTransactionNonce", accountKey.Address)
	if err != nil {
		log.Error("get tx nonce error", "err", err)
		return err
	}
	nonce = (uint64)(hexNonce)
	log.Info("get nonce ok", "nonce", nonce)

	var tx *types.Transaction
	tx = types.NewTransaction((uint64)(nonce), toAddr, big.NewInt(0), 1e9, common.Big1, actionSlice, true)
	tx, err = types.SignTx(tx, signer, accountKey.PrivKey)
	if err != nil {
		log.Error("sign tx error", "err", err)
		return err
	}

	err = sendTxToRpc(tx, client)
	if err != nil {
		return err
	}

	err = retrieveRspFromRpc(tx, client)
	return err
}
