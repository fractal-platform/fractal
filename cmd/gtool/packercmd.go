package main

/*
#cgo CFLAGS: -I../../transaction/txexec
#cgo LDFLAGS: -L../../transaction/txexec -lwasmlib
#include <stdlib.h>

void gen_action_bytes(unsigned char *abiBytes, int abiLength, unsigned char *funcNameBytes, int funcNameLength, unsigned char *jsonBytes, int jsonLength, void** actionBytes, int* actionLength);
*/
import "C"
import (
	"bytes"
	"io/ioutil"
	"math/big"
	"path"
	"strconv"
	"unsafe"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rpc/client"
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
	abi, _ := ioutil.ReadFile(abiFile)

	packerId := uint32(ctx.GlobalUint64(PackerIdFlag.Name))
	packerRpcAddressString := ctx.GlobalString(PackerRpcAddressFlag.Name)
	packerCoinbaseString := ctx.GlobalString(PackerCoinbaseFlag.Name)
	packerPubKeyString := ctx.GlobalString(PackerPubKeyFlag.Name)

	buffer := bytes.NewBufferString("[")
	buffer.WriteString(strconv.Itoa(int(packerId)) + ",")
	buffer.WriteString("\"" + packerRpcAddressString + "\",")
	buffer.WriteString("\"" + packerCoinbaseString[2:] + "\",")

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

	args := []byte(argsString)
	action := []byte("setkey")
	var actionBytes unsafe.Pointer
	var actionLength C.int
	var actionSlice []byte
	C.gen_action_bytes((*C.uchar)(&abi[0]), C.int(len(abi)), (*C.uchar)(&action[0]), C.int(len(action)), (*C.uchar)(&args[0]), C.int(len(args)), &actionBytes, (*C.int)(&actionLength))
	for i := C.int(0); i < actionLength; i++ {
		ptr := (*C.uchar)(unsafe.Pointer(uintptr(actionBytes) + uintptr(i)))
		actionSlice = append(actionSlice, byte(*ptr))
	}
	C.free(actionBytes)
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
