/*
1. put libwasmlib.dylib into .../fractal/transaction/txexec
2. export DYLD_LIBRARY_PATH=.../fractal/transaction/txexec

use case: wasmtest --wasm ./floattest.wasm --action test --abi ./floattest.abi --args "[1.0, 1.1]" exec
*/
package main

import "C"
import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/fractal-platform/fractal/cmd/utils"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	w "github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/transaction/txexec"
	utils2 "github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/abi"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"os"
	"sort"
)

const (
	clientIdentifier = "wasmtest" // Client identifier to advertise over the network
)

var (
	// flags
	WasmPathFlag = cli.StringFlag{
		Name:  "wasm",
		Usage: "wasm file path",
	}
	AbiPathFlag = cli.StringFlag{
		Name:  "abi",
		Usage: "abi file path",
	}
	ActionFlag = cli.StringFlag{
		Name:  "action",
		Usage: "action name",
	}
	ArgsFlag = cli.StringFlag{
		Name:  "args",
		Usage: "action args",
	}

	execCommand = cli.Command{
		Name:        "exec",
		Usage:       "exec one",
		Description: `exec one`,
		Action:      exec,
	}

	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit   = ""
	versionMeta = "unstable" // Version metadata to append to the version string
	// The app that holds all commands and flags.
	app = utils.NewApp(versionMeta, gitCommit, "the wasmtest command line interface")
	// flags that configure the node
	txtestFlags = []cli.Flag{
		WasmPathFlag,
		AbiPathFlag,
		ActionFlag,
		ArgsFlag,
	}
)

func init() {
	// Initialize the CLI app and start Geth
	app.Action = nil
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2013-2018 The go-fractal Authors"
	app.Commands = []cli.Command{
		execCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, txtestFlags...)
}

func main() {
	log.SetDefaultLogger(log.InitLog15Logger(log.LvlDebug, os.Stdout))
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// txsend is the main entry point for tx send test
func exec(ctx *cli.Context) error {
	wasm := ctx.GlobalString(WasmPathFlag.Name)
	abiPath := ctx.GlobalString(AbiPathFlag.Name)
	action := ctx.GlobalString(ActionFlag.Name)
	args := ctx.GlobalString(ArgsFlag.Name)

	db := dbwrapper.NewMemDatabase()
	stateCache := state.NewDatabase(db)
	stateDb, _ := state.New(common.Hash{}, stateCache)
	root := stateDb.IntermediateRoot(true)
	log.Info("WASM TEST", "root", root)

	// get params
	mockBlock := &types.Block{
		Header: types.BlockHeader{
			Round:  1e7,
			Height: 1e6,
		},
	}
	mockAddr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	mockAddr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	mockGas := uint64(1e10)
	callbackParamKey := w.GetGlobalRegisterParam().RegisterParam(stateDb, mockBlock)

	log.Info("WASM TEST", "simpleHash", mockBlock.SimpleHash(), "fullHash", mockBlock.FullHash())

	code, _ := ioutil.ReadFile(wasm)
	codeLength := len(code)
	log.Info("WASM TEST", "wasm", wasm, "codeLength", codeLength, "code", hexutil.Encode(code))

	abidef, _ := ioutil.ReadFile(abiPath)
	abidefLength := len(abidef)
	log.Info("WASM TEST", "abi", abiPath, "abidefLength", abidefLength)

	actionUint, err := utils2.String2Uint64(action)
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
	err = serializer.Serialize(argsData, action, writer)
	if err != nil {
		log.Error("serialize args failed", "err", err)
		return err
	}
	actionSlice := writer.Bytes()
	log.Info("WASM TEST", "action", actionSlice, "length", len(actionSlice))

	// DO
	result := txexec.CallWasmContract(code, actionSlice, mockAddr1, mockAddr2, mockAddr1, mockAddr1, 10e6, false, false, &mockGas, callbackParamKey)
	if result != 0 {
		log.Error("CallWasmContract returned with error", "result", result)
	}
	log.Info("remain gas", "gas", mockGas)

	w.GetGlobalRegisterParam().UnRegisterParam(callbackParamKey)
	root = stateDb.IntermediateRoot(true)
	log.Info("WASM TEST", "root", root)

	return nil
}
