package main

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/pkg/errors"
	"gopkg.in/urfave/cli.v1"
)

var (
	miningKeySubFolder = "mining_keys"
	packerKeySubFolder = "packer_keys"
)

var (
	keysCommand = cli.Command{
		Name:  "keys",
		Usage: "Manage Fractal Keys",
		Flags: []cli.Flag{
			KeyFolderFlag,
			PasswordFlag,
			AddressFlag,
			RpcFlag,
			ChainIdFlag,
		},
		Subcommands: []cli.Command{
			{
				Name:   "list",
				Usage:  "List Fractal Keys",
				Action: listKeys,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
				},
			},
			{
				Name:   "newkeys",
				Usage:  "New Keys for mining/packer/account",
				Action: newKeys,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
				},
			},
			{
				Name:   "newminingkey",
				Usage:  "New Mining Key",
				Action: newMiningKey,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
					AddressFlag,
				},
			},
			{
				Name:   "regminingkey",
				Usage:  "Register Mining Key",
				Action: registerMiningKey,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
					RpcFlag,
					ChainIdFlag,
				},
			},
			{
				Name:   "newpackerkey",
				Usage:  "New Packer Key",
				Action: newPackerKey,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
					AddressFlag,
				},
			},
			{
				Name:   "export",
				Usage:  "Export Private Key",
				Action: exportPrivateKey,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
				},
			},
			{
				Name:   "newcheckpointkey",
				Usage:  "New Check Point Key",
				Action: newCheckPointKey,
				Flags: []cli.Flag{
					KeyFolderFlag,
					PasswordFlag,
				},
			},
		},
	}
)

func listKeys(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)

	if folder == "" {
		return errors.New("key folder must be set")
	}

	accountKeyFile := path.Join(folder, "account.json")
	accountKey, err := keys.LoadAccountKey(accountKeyFile, password)
	if err != nil {
		//return err
	} else {
		fmt.Printf("Account Key Address: %s\n", hexutil.Encode(accountKey.Address[:]))
	}

	miningKeyManager := keys.NewMiningKeyManager(path.Join(folder, miningKeySubFolder), password)
	miningKeyManager.Load()
	for k, v := range miningKeyManager.Keys() {
		fmt.Printf("Mining Key Address: %s\n", hexutil.Encode(k[:]))
		for pub := range v {
			fmt.Printf("Mining Public Key: %s\n", hexutil.Encode(pub[:]))
		}
	}

	packerKeyManager := keys.NewPackerKeyManager(path.Join(folder, packerKeySubFolder), password)
	if packerKeyManager.Load() != nil {
		panic("unlock password error")
	}
	for k, v := range packerKeyManager.Keys() {
		fmt.Printf("Packer Key Address: %s\n", hexutil.Encode(k[:]))
		for pub := range v {
			fmt.Printf("Packer Public Key: %s\n", hexutil.Encode(pub[:]))
		}
	}

	return nil
}

func newKeys(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)

	if folder == "" {
		return errors.New("key folder must be set")
	}
	if password == "" {
		return errors.New("password must be set")
	}

	// create if not exists
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		os.MkdirAll(folder, 0755)
	} else {
		return errors.New("key already exist: " + folder)
	}

	accountKeyFile := path.Join(folder, "account.json")
	accountKey := keys.CreateAccountKey(accountKeyFile, password)
	fmt.Printf("New Account Key Address: %s\n", hexutil.Encode(accountKey.Address[:]))

	miningKeyManager := keys.NewMiningKeyManager(path.Join(folder, miningKeySubFolder), password)
	miningPub := miningKeyManager.CreateKey(accountKey.Address)
	fmt.Printf("New Mining Key Address: %s\n", hexutil.Encode(accountKey.Address[:]))
	fmt.Printf("New Mining Public Key: %s\n", hexutil.Encode(miningPub.Marshal()[:]))

	packerKeyManager := keys.NewPackerKeyManager(path.Join(folder, packerKeySubFolder), password)
	packerPub := packerKeyManager.CreateKey(accountKey.Address)
	fmt.Printf("New Packer Key Address: %s\n", hexutil.Encode(accountKey.Address[:]))
	fmt.Printf("New Packer Public Key: %s\n", hexutil.Encode(packerPub.Marshal()[:]))

	return nil
}

func newMiningKey(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)
	address := ctx.GlobalString(AddressFlag.Name)
	addr := common.HexToAddress(address)

	if folder == "" {
		return errors.New("key folder must be set")
	}
	if address == "" {
		return errors.New("address must be set")
	}

	miningKeyManager := keys.NewMiningKeyManager(path.Join(folder, miningKeySubFolder), password)
	pub := miningKeyManager.CreateKey(addr)
	fmt.Printf("New Mining Key Address: %s\n", hexutil.Encode(addr[:]))
	fmt.Printf("New Mining Public Key: %s\n", hexutil.Encode(pub.Marshal()[:]))

	return nil
}

func registerMiningKey(ctx *cli.Context) error {
	initLogger(ctx)

	rpc := ctx.GlobalString(RpcFlag.Name)

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
	miningKeyPath := path.Join(folder, "mining_keys")
	miningKeyManager := keys.NewMiningKeyManager(miningKeyPath, password)
	miningKeyManager.Load()
	miningKeys := miningKeyManager.Keys()[accountKey.Address]

	// mining pub key
	if len(miningKeys) <= 0 {
		log.Error("no mining keys")
		return errors.New("no mining keys")
	}
	var miningPubKey keys.MiningPubkey
	for k := range miningKeys {
		miningPubKey = k
		break
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

	// generate action
	actionSlice := make([]byte, 8+2+crypto.BlsPubkeyLen)
	actionName, _ := utils.String2Uint64("setkey")
	binary.LittleEndian.PutUint64(actionSlice[:8], actionName)
	actionSlice[8] = crypto.BlsPubkeyLen // two bytes for len
	actionSlice[9] = 1                   // two bytes for len
	copy(actionSlice[10:], miningPubKey[:])
	log.Info("action slice", "data", hexutil.Encode(actionSlice))

	// sign tx
	tx := types.NewTransaction((uint64)(nonce), common.HexToAddress(params.MinerKeyContractAddr), big.NewInt(0), 1e9, common.Big1, actionSlice, true)
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

func newPackerKey(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)
	address := ctx.GlobalString(AddressFlag.Name)
	addr := common.HexToAddress(address)

	if folder == "" {
		return errors.New("key folder must be set")
	}
	if address == "" {
		return errors.New("address must be set")
	}

	packerKeyManager := keys.NewPackerKeyManager(path.Join(folder, packerKeySubFolder), password)
	pub := packerKeyManager.CreateKey(addr)
	fmt.Printf("New Packer Key Address: %s\n", hexutil.Encode(addr[:]))
	fmt.Printf("New Packer Public Key: %s\n", hexutil.Encode(pub.Marshal()[:]))

	return nil
}

func exportPrivateKey(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)

	if folder == "" {
		return errors.New("key folder must be set")
	}

	accountKeyFile := path.Join(folder, "account.json")
	accountKey, err := keys.LoadAccountKey(accountKeyFile, password)
	if err != nil {
		//return err
	} else {
		fmt.Printf("Account Address: %s\n", hexutil.Encode(accountKey.Address[:]))
	}
	fmt.Printf("Account Private Key: %s\n", hexutil.Encode(accountKey.PrivKey.Marshal()))

	return nil
}

func newCheckPointKey(ctx *cli.Context) error {
	initLogger(ctx)

	folder := ctx.GlobalString(KeyFolderFlag.Name)
	password := ctx.GlobalString(PasswordFlag.Name)

	if folder == "" {
		return errors.New("key folder must be set")
	}
	if password == "" {
		return errors.New("password must be set")
	}

	// create if not exists
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		os.MkdirAll(folder, 0755)
	}

	checkPointKeyFile := path.Join(folder, "check_point_key.json")
	priKey := keys.CreateCheckPointKey(checkPointKeyFile, password)
	pubKey := priKey.Public()
	pubKeyByte := pubKey.Marshal()
	fmt.Println("Create Check Point Key Ok")
	fmt.Printf("Check Point Public Key: %s\n", hexutil.Encode(pubKeyByte))

	return nil
}
