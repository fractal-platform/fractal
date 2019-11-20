package fractalsdk

import (
	"math/big"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/rlp"
)

type txSender struct {
	*chainReader

	signer        types.Signer
	accountPriKey crypto.PrivateKey
	accountAddr   string
	nonce         uint64
	sendTxMu      sync.Mutex
}

func (t *txSender) SendTransactionRaw(nonce uint64, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) (string, error) {
	t.sendTxMu.Lock()
	defer t.sendTxMu.Unlock()

	return t.sendTransaction(nonce, to, amount, gasLimit, gasPrice, data, broadcast)
}

func (t *txSender) sendTransaction(nonce uint64, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) (string, error) {
	var (
		addrTo common.Address
		tx     *types.Transaction
		err    error
		txHash common.Hash
	)
	if to == "" {
		tx = types.NewContractCreation(nonce, amount, gasLimit, gasPrice, data, broadcast)
	} else {
		addrTo = common.HexToAddress(to)
		tx = types.NewTransaction(nonce, addrTo, amount, gasLimit, gasPrice, data, broadcast)
	}

	tx, err = types.SignTx(tx, t.signer, t.accountPriKey)
	if err != nil {
		return "", err
	}
	encodedTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return "", err
	}

	err = t.call(&txHash, "txpool_sendRawTransaction", (hexutil.Bytes)(encodedTx))
	if err != nil {
		return "", err
	}
	return txHash.String(), nil
}

func (t *txSender) SendTransactionSync(to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool, timeout time.Duration) (*TransactionDetails, error) {
	t.sendTxMu.Lock()
	defer t.sendTxMu.Unlock()

	nonce, err := t.GetTransactionNonce(t.accountAddr)
	if err != nil {
		return nil, err
	}

	if nonce > t.nonce {
		t.nonce = nonce
	}

	txHash, err := t.sendTransaction(t.nonce, to, amount, gasLimit, gasPrice, data, broadcast)
	if err != nil {
		return nil, err
	}
	t.nonce++

	var (
		resultChan = make(chan *ExecResult)
		interval   = 1
	)

	var timeoutSig = time.After(timeout)

	go func() {
		for {
			select {
			case <-timeoutSig:
				resultChan <- &ExecResult{
					TxDetails: nil,
					Err:       ErrTransactionNotExecuted,
				}
				return
			default:
				time.Sleep(time.Duration(interval) * time.Second)
				txDetails, err := t.GetTransactionByHash(txHash)
				if err != nil {
					resultChan <- &ExecResult{
						TxDetails: nil,
						Err:       err,
					}
					return
				}
				if txDetails != nil {
					if txDetails.Receipt != nil {
						resultChan <- &ExecResult{
							TxDetails: txDetails,
							Err:       nil,
						}
						return
					}
				}
			}
		}
	}()

	result := <-resultChan
	return result.TxDetails, result.Err
}

func (t *txSender) SendTransactionAsync(to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool, timeout time.Duration, resultChan chan<- *ExecResult) error {
	t.sendTxMu.Lock()
	defer t.sendTxMu.Unlock()

	nonce, err := t.GetTransactionNonce(t.accountAddr)
	if err != nil {
		return err
	}

	if nonce > t.nonce {
		t.nonce = nonce
	}

	txHash, err := t.sendTransaction(t.nonce, to, amount, gasLimit, gasPrice, data, broadcast)
	if err != nil {
		return err
	}
	t.nonce++

	var interval = 1

	var timeoutSig = time.After(timeout)

	go func() {
		for {
			select {
			case <-timeoutSig:
				resultChan <- &ExecResult{
					TxDetails: nil,
					Err:       ErrTransactionNotExecuted,
				}
				return
			default:
				time.Sleep(time.Duration(interval) * time.Second)
				txDetails, err := t.GetTransactionByHash(txHash)
				if err != nil {
					resultChan <- &ExecResult{
						TxDetails: nil,
						Err:       err,
					}
					return
				}
				if txDetails != nil {
					if txDetails.Receipt != nil {
						resultChan <- &ExecResult{
							TxDetails: txDetails,
							Err:       nil,
						}
						return
					}
				}
			}
		}
	}()

	return nil
}

func (t *txSender) BatchSendTransaction(taskChanContainer <-chan chan *ExecTask) error {
	t.sendTxMu.Lock()
	defer t.sendTxMu.Unlock()

	nonce, err := t.GetTransactionNonce(t.accountAddr)
	if err != nil {
		return err
	}

	if nonce > t.nonce {
		t.nonce = nonce
	}

	var nonceMu sync.Mutex
	var interval = 1

	go func() {
		for taskChan := range taskChanContainer {
			task := <-taskChan

			nonceMu.Lock()
			txHash, err := t.sendTransaction(t.nonce, task.To, task.Amount, task.GasLimit, task.GasPrice, task.Data, task.Broadcast)
			if err != nil {
				task.TxDetails = nil
				task.Err = err
				taskChan <- task
			}
			t.nonce++
			nonceMu.Unlock()

			var secondWaitForExec = task.SecondWaitForExec
			if secondWaitForExec < 20 {
				secondWaitForExec = 20
			}

			var timeout = time.After(time.Duration(secondWaitForExec) * time.Second)

			go func(taskChan chan *ExecTask) {
				for {
					select {
					case <-timeout:
						task.TxDetails = nil
						task.Err = ErrTransactionNotExecuted
						taskChan <- task
						return
					default:
						time.Sleep(time.Duration(interval) * time.Second)
						txDetails, err := t.GetTransactionByHash(txHash)
						if err != nil {
							task.TxDetails = nil
							task.Err = err
							taskChan <- task
							return
						}
						if txDetails != nil {
							if txDetails.Receipt != nil {
								task.TxDetails = txDetails
								task.Err = nil
								taskChan <- task
								return
							}
						}
					}
				}
			}(taskChan)
		}
	}()

	return nil
}
