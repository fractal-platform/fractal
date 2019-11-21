package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/ftl/api"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/abi"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	txCommand = cli.Command{
		Name:  "tx",
		Usage: "Generate Transaction",
		Flags: []cli.Flag{
			RpcFlag,
			PackerFlag,
			ToFlag,
			ValueFlag,
			TpsFlag,
			NProcessFlag,
			ChainIdFlag,
			KeyFolderFlag,
			PasswordFlag,
			WasmFlag,
			AbiFlag,
			ActionFlag,
			ArgsFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "send",
				Usage:  "Send Transaction",
				Action: sendTransaction,
				Flags: []cli.Flag{
					RpcFlag,
					PackerFlag,
					ToFlag,
					ValueFlag,
					ChainIdFlag,
					KeyFolderFlag,
					PasswordFlag,
				},
			},
			{
				Name:   "batch",
				Usage:  "Batch Send Transaction",
				Action: batchSendTransaction,
				Flags: []cli.Flag{
					RpcFlag,
					PackerFlag,
					ToFlag,
					ValueFlag,
					ChainIdFlag,
					KeyFolderFlag,
					PasswordFlag,
					TpsFlag,
					NProcessFlag,
				},
			},
			{
				Name:   "deploy",
				Usage:  "Deploy Contract",
				Action: deployContract,
				Flags: []cli.Flag{
					RpcFlag,
					ValueFlag,
					PackerFlag,
					ChainIdFlag,
					KeyFolderFlag,
					PasswordFlag,
					WasmFlag,
				},
			},
			{
				Name:   "call",
				Usage:  "Call Contract",
				Action: callContract,
				Flags: []cli.Flag{
					RpcFlag,
					PackerFlag,
					ChainIdFlag,
					KeyFolderFlag,
					PasswordFlag,
					ToFlag,
					ValueFlag,
					AbiFlag,
					ActionFlag,
					ArgsFlag,
				},
			},
		},
	}
)

func sendTxToRpc(tx *types.Transaction, client *rpcclient.Client) error {
	bytes, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Error("encode tx error", "err", err)
		return err
	}

	err = client.Call(nil, "txpool_sendRawTransaction", (hexutil.Bytes)(bytes))
	if err != nil {
		fmt.Println("send tx error:", err)
		return err
	}
	//log.Info("send tx success", "hash", tx.Hash())
	return nil
}

func retrieveRspFromRpc(tx *types.Transaction, client *rpcclient.Client) error {
	var txrsp *api.RPCTransaction
	for {
		time.Sleep(2 * time.Second)
		err := client.Call(&txrsp, "txpool_getTransactionByHash", tx.Hash())
		if err != nil {
			fmt.Println("get tx rsp error:", err)
			return err
		}
		if txrsp != nil {
			log.Info("recv tx rsp", "from", txrsp.From, "nonce", uint64(txrsp.Nonce), "hash", txrsp.Hash, "to", txrsp.To, "receipt", txrsp.Receipt)
			if txrsp.Receipt != nil {
				break
			}
		} else {
			log.Info("recv tx rsp", "txrsp", txrsp)
		}
	}
	return nil
}

func sendTransaction(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	packer := ctx.GlobalBool(PackerFlag.Name)
	to := ctx.GlobalString(ToFlag.Name)
	value := ctx.GlobalInt64(ValueFlag.Name)

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

	var addrTo = common.HexToAddress(to)
	var tx *types.Transaction
	if packer {
		tx = types.NewTransaction((uint64)(nonce), addrTo, big.NewInt(value), 3e6, common.Big1, []byte{}, false)
	} else {
		tx = types.NewTransaction((uint64)(nonce), addrTo, big.NewInt(value), 3e6, common.Big1, []byte{}, true)
	}
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

func batchSendTransaction(ctx *cli.Context) error {
	initLogger(ctx)

	tps := ctx.GlobalInt(TpsFlag.Name)
	nprocess := ctx.GlobalInt(NProcessFlag.Name)
	rpc := ctx.GlobalString(RpcFlag.Name)
	packer := ctx.GlobalBool(PackerFlag.Name)
	to := ctx.GlobalString(ToFlag.Name)
	value := ctx.GlobalInt64(ValueFlag.Name)

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

	var addrTo = common.HexToAddress(to)
	lastTime := time.Now().UnixNano()
	for i := 0; i < nprocess; i++ {
		go func(index int) {
			// Connect the client.
			client, err := rpcclient.Dial(rpc)
			if err != nil {
				log.Error("connect to rpc error", "rpc", rpc)
				return
			}

			ticker := time.NewTicker(time.Nanosecond * time.Duration(1e9*nprocess/tps))
			for range ticker.C {
				currentNonce := atomic.AddUint64(&nonce, 1)
				var tx *types.Transaction
				if packer {
					tx = types.NewTransaction(currentNonce, addrTo, big.NewInt(value), 3e6, common.Big1, []byte{}, false)
				} else {
					tx = types.NewTransaction(currentNonce, addrTo, big.NewInt(value), 3e6, common.Big1, []byte{}, true)
				}
				tx, err = types.SignTx(tx, signer, accountKey.PrivKey)
				if err != nil {
					log.Error("sign tx error", "err", err)
					return
				}

				err = sendTxToRpc(tx, client)
				if err != nil {
					continue
				}
			}

		}(i)
	}

	var lastNonce uint64 = nonce
	for {
		currentTime := time.Now().UnixNano()
		if currentTime-lastTime > 1e9 {
			log.Info("generating transaction", "tps", nonce-lastNonce)
			lastTime = currentTime
			lastNonce = nonce
		}
		time.Sleep(time.Millisecond)
	}

}

func deployContract(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	packer := ctx.GlobalBool(PackerFlag.Name)
	value := ctx.GlobalInt64(ValueFlag.Name)

	wasm := ctx.GlobalString(WasmFlag.Name)
	code, _ := ioutil.ReadFile(wasm)

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
	if packer {
		tx = types.NewContractCreation((uint64)(nonce), big.NewInt(value), 1e9, common.Big1, code, false)
	} else {
		tx = types.NewContractCreation((uint64)(nonce), big.NewInt(value), 1e9, common.Big1, code, true)
	}
	tx, err = types.SignTx(tx, signer, accountKey.PrivKey)
	if err != nil {
		log.Error("sign tx error", "err", err)
		return err
	}

	err = sendTxToRpc(tx, client)
	if err != nil {
		return err
	}

	contractAddr := crypto.CreateAddress(accountKey.Address, nonce)
	log.Info("deploy contract over", "hash", tx.Hash(), "contract", contractAddr)

	err = retrieveRspFromRpc(tx, client)
	return err
}

func callContract(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)
	packer := ctx.GlobalBool(PackerFlag.Name)
	value := ctx.GlobalInt64(ValueFlag.Name)

	abiFile := ctx.GlobalString(AbiFlag.Name)
	abidef, _ := ioutil.ReadFile(abiFile)
	action := ctx.GlobalString(ActionFlag.Name)
	args := ctx.GlobalString(ArgsFlag.Name)

	actionUint, err := utils.String2Uint64(action)
	if err != nil {
		log.Error("parse action failed", "err", err)
		return err
	}
	actionBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(actionBytes, actionUint) // use LittleEndian

	var argsData interface{}
	err = json.Unmarshal([]byte(args), &argsData)
	if err != nil {
		log.Error("unmarshal args failed", "err", err)
		return err
	}

	writer := bytes.NewBuffer(actionBytes)
	serializer, err := abi.NewAbiSerializer(string(abidef))
	if err != nil {
		log.Error("callContract NewAbiSerializer failed", "err", err)
		return err
	}
	err = serializer.Serialize(argsData, action, writer)
	if err != nil {
		log.Error("serialize args failed", "err", err)
		return err
	}
	actionSlice := writer.Bytes()
	log.Info("generate action bytes ok", "action", hexutil.Encode(actionSlice), "length", len(actionSlice))

	to := ctx.GlobalString(ToFlag.Name)
	toAddr := common.HexToAddress(to)

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
	if packer {
		tx = types.NewTransaction((uint64)(nonce), toAddr, big.NewInt(value), 1e9, common.Big1, actionSlice, false)
	} else {
		tx = types.NewTransaction((uint64)(nonce), toAddr, big.NewInt(value), 1e9, common.Big1, actionSlice, true)
	}
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
