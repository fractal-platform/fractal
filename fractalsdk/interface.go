package fractalsdk

import (
	"errors"
	"math/big"
	"time"

	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/keys"
)

type SignerTypeEnum byte

const (
	FakeSigner SignerTypeEnum = iota
	Eip155Signer
)

var (
	ErrNotDial                = errors.New("Should dial first")
	ErrTransactionNotExecuted = errors.New("The transaction is not executed")
)

type Logger interface {
	Info(s string)
	Error(s string)
}

type ChainReader interface {
	DialHttp(rawUrl string) error
	DialWs(rawUrl string) error

	GetCurrentBalance(address string) (*big.Int, error)
	GetBalance(address string, blockFullHash string) (*big.Int, error)
	GetCurrentCode(address string) (string, error)
	GetCode(address string, blockFullHash string) (string, error)
	GetCurrentStorage(address string, table string, key string) (string, error)
	GetStorage(address string, table string, key string, blockFullHash string) (string, error)
	GetContractOwner(contractAddr string) (string, error)
	GetGenesis() (*Block, error)
	GetBlock(blockFullHash string) (*Block, error)
	GetHeadBlock() (*Block, error)
	GetBlockByHeight(height uint64) (*Block, error)
	GetBackwardBlocks(blockFullHash string, count uint32) ([]*Block, error)
	GetAncestorBlocks(blockFullHash string, count uint32) ([]*Block, error)
	GetDescendantBlocks(blockFullHash string, count uint32) ([]*Block, error)
	GetNearbyBlocks(blockFullHash string, width uint32) ([]*Block, error)
	GetTxPackageByHash(pkgHash string) (*TxPackage, error)
	GetTransactionNonce(address string) (uint64, error)
	GetTransactionByHash(hash string) (*TransactionDetails, error)
	Call(from string, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (CallResult, error)

	SubNewBlock(unsubscribe <-chan struct{}, blockCh chan *Block) error
}

type TxSender interface {
	ChainReader
	SendTransactionRaw(nonce uint64, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) (string, error)
	SendTransactionSync(to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool, timeout time.Duration) (*TransactionDetails, error)
	SendTransactionAsync(to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool, timeout time.Duration, resultChan chan<- *ExecResult) error
	BatchSendTransaction(taskChanContainer <-chan chan *ExecTask) error
}

func NewChainReader(logger Logger) ChainReader {
	c := &chainReader{
		logger: logger,
	}
	return c
}

func NewTxSender(logger Logger, chainId uint64, signerType SignerTypeEnum, priKeyPath string, password string) (TxSender, error) {
	t := &txSender{
		chainReader: &chainReader{
			logger: logger,
		},
	}

	switch signerType {
	case FakeSigner:
		t.signer = types.NewFakeSigner()
	case Eip155Signer:
		t.signer = types.NewEIP155Signer(chainId)
	default:
		t.signer = types.NewEIP155Signer(chainId)
	}

	account, err := keys.LoadAccountKey(priKeyPath, password)
	if err != nil {
		return nil, err
	}
	t.accountPriKey = account.PrivKey
	t.accountAddr = account.Address.String()
	return t, nil
}
