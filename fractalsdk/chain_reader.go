package fractalsdk

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/ftl/api"
	"github.com/fractal-platform/fractal/rpc/client"
)

type chainReader struct {
	logger Logger

	httpConn *rpcclient.Client
	wsConn   *rpcclient.Client
}

func (c *chainReader) info(s string) {
	if c.logger != nil && !reflect.ValueOf(c.logger).IsNil() {
		c.logger.Info(s)
	}
}

func (c *chainReader) error(s string) {
	if c.logger != nil && !reflect.ValueOf(c.logger).IsNil() {
		c.logger.Error(s)
	}
}

func (c *chainReader) DialHttp(rawUrl string) error {
	var err error
	c.httpConn, err = rpcclient.Dial(rawUrl)
	return err
}

func (c *chainReader) DialWs(rawUrl string) error {
	var err error
	c.wsConn, err = rpcclient.Dial(rawUrl)
	return err
}

func (c *chainReader) call(result interface{}, method string, args ...interface{}) error {
	if c.httpConn == nil {
		return ErrNotDial
	}
	err := c.httpConn.Call(result, method, args...)
	if err != nil {
		return err
	}
	return nil
}

func (c *chainReader) subscribe(unsubscribe <-chan struct{}, namespace string, channel interface{}, args ...interface{}) error {
	if c.wsConn == nil {
		return ErrNotDial
	}

	var reConnectTimes = 10
	var reConnectInterval = 10 * time.Second
	var timeOut = 10 * time.Second

	go func() {
		for i := 0; i < reConnectTimes; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeOut)
			subscribe, err := c.wsConn.Subscribe(ctx, namespace, channel, args...)
			if err != nil {
				c.error(fmt.Sprintf("Subscription error(%d times): %s", i+1, err.Error()))
				cancel()
				time.Sleep(reConnectInterval)
				continue
			} else {
				// refresh retry times
				i = 0
				c.info(fmt.Sprintf("Subscription success"))
			}

			select {
			case <-unsubscribe:
				c.info(fmt.Sprintf("Subscription stopped"))
				subscribe.Unsubscribe()
				cancel()
				return
			case err := <-subscribe.Err():
				c.error(fmt.Sprintf("Subscription connection lost(%d times): %s", i+1, err.Error()))
				cancel()
				time.Sleep(reConnectInterval)
			}
		}
		c.error(fmt.Sprintf("Subscription failed"))
	}()
	return nil
}

func (c *chainReader) GetCurrentBalance(address string) (*big.Int, error) {
	var balance *hexutil.Big
	err := c.call(&balance, "ftl_getBalance", address, "latest")
	if err != nil {
		return nil, err
	}
	return (*big.Int)(balance), nil
}

func (c *chainReader) GetBalance(address string, blockFullHash string) (*big.Int, error) {
	var balance *hexutil.Big
	err := c.call(&balance, "ftl_getBalance", address, blockFullHash)
	if err != nil {
		return nil, err
	}
	return (*big.Int)(balance), nil
}

func (c *chainReader) GetCurrentCode(address string) (string, error) {
	var code string
	err := c.call(&code, "ftl_getCode", address, "latest")
	return code, err
}

func (c *chainReader) GetCode(address string, blockFullHash string) (string, error) {
	var code string
	err := c.call(&code, "ftl_getCode", address, blockFullHash)
	return code, err
}

func (c *chainReader) GetCurrentStorage(address string, table string, key string) (string, error) {
	var storage string
	err := c.call(&storage, "ftl_getStorageAt", address, table, key, "latest")
	return storage, err
}

func (c *chainReader) GetStorage(address string, table string, key string, blockFullHash string) (string, error) {
	var storage string
	err := c.call(&storage, "ftl_getStorageAt", address, table, key, blockFullHash)
	return storage, err
}

func (c *chainReader) GetContractOwner(contractAddr string) (string, error) {
	var owner string
	err := c.call(&owner, "ftl_getContractOwner", contractAddr)
	return owner, err
}

func (c *chainReader) GetGenesis() (*Block, error) {
	var block *Block
	err := c.call(&block, "ftl_genesis")
	return block, err
}

func (c *chainReader) GetBlock(blockFullHash string) (*Block, error) {
	var block *Block
	err := c.call(&block, "ftl_getBlock", blockFullHash)
	return block, err
}

func (c *chainReader) GetHeadBlock() (*Block, error) {
	var block *Block
	err := c.call(&block, "ftl_headBlock")
	return block, err
}

func (c *chainReader) GetBlockByHeight(height uint64) (*Block, error) {
	var block *Block
	err := c.call(&block, "ftl_getBlockByHeight", hexutil.Uint64(height))
	return block, err
}

func (c *chainReader) GetBackwardBlocks(blockFullHash string, count uint32) ([]*Block, error) {
	var blocks []*Block
	err := c.call(&blocks, "ftl_getBackwardBlocks", blockFullHash, count)
	return blocks, err
}

func (c *chainReader) GetAncestorBlocks(blockFullHash string, count uint32) ([]*Block, error) {
	var blocks []*Block
	err := c.call(&blocks, "ftl_getAncestorBlocks", blockFullHash, count)
	return blocks, err
}

func (c *chainReader) GetDescendantBlocks(blockFullHash string, count uint32) ([]*Block, error) {
	var blocks []*Block
	err := c.call(&blocks, "ftl_getDescendantBlocks", blockFullHash, count)
	return blocks, err
}

func (c *chainReader) GetNearbyBlocks(blockFullHash string, width uint32) ([]*Block, error) {
	var blocks []*Block
	err := c.call(&blocks, "ftl_getNearbyBlocks", blockFullHash, width)
	return blocks, err
}

func (c *chainReader) GetTxPackageByHash(pkgHash string) (*TxPackage, error) {
	var txPackage *TxPackage
	err := c.call(&txPackage, "pack_getTxPackageByHash", pkgHash)
	return txPackage, err
}

func (c *chainReader) GetTransactionNonce(address string) (uint64, error) {
	var hexNonce *hexutil.Uint64
	err := c.call(&hexNonce, "txpool_getTransactionNonce", address)
	if err != nil {
		return 0, err
	}
	return uint64(*hexNonce), err
}

func (c *chainReader) GetTransactionByHash(hash string) (*TransactionDetails, error) {
	var transactionDetails *TransactionDetails
	err := c.call(&transactionDetails, "txpool_getTransactionByHash", hash)
	return transactionDetails, err
}

func (c *chainReader) SubNewBlock(unsubscribe <-chan struct{}, blockCh chan *Block) error {
	return c.subscribe(unsubscribe, "ftl", blockCh, "subNewBlock")
}

func (c *chainReader) Call(from string, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (CallResult, error) {
	var (
		args   api.SendTxArgs
		result CallResult
		nonce  uint64
	)

	args.From = common.HexToAddress(from)
	if to == "" {
		args.To = nil
	} else {
		toAddr := common.HexToAddress(to)
		args.To = &toAddr
	}
	args.Gas = (*hexutil.Uint64)(&gasLimit)
	args.GasPrice = (*hexutil.Big)(gasPrice)
	args.Value = (*hexutil.Big)(gasPrice)
	args.Nonce = (*hexutil.Uint64)(&nonce)
	args.Data = (*hexutil.Bytes)(&data)

	err := c.call(&result, "txpool_call", args)
	return result, err
}
