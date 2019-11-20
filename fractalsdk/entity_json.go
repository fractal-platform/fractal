package fractalsdk

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
)

func (b *Block) UnmarshalJSON(input []byte) error {
	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return err
	}

	m := data.(map[string]interface{})
	b.FullHash = m["FullHash"].(string)
	b.SimpleHash = m["SimpleHash"].(string)

	bh, _ := json.Marshal(m["Header"])
	bb, _ := json.Marshal(m["Body"])

	type BlockHeader struct {
		ParentHash     *common.Hash    `json:"parentHash"`
		Round          *uint64         `json:"round"`
		Sig            []byte          `json:"sig"`
		Coinbase       *common.Address `json:"miner"`
		Difficulty     *big.Int        `json:"difficulty"`
		Height         *uint64         `json:"height"`
		Amount         *uint64         `json:"amount"`
		GasLimit       *uint64         `json:"gasLimit"`
		GasUsed        *uint64         `json:"gasUsed"`
		StateHash      *common.Hash    `json:"stateHash"`
		TxHash         *common.Hash    `json:"txHash"`
		ReceiptHash    *common.Hash    `json:"receiptsRoot"`
		ParentFullHash *common.Hash    `json:"parentFullHash"`
		Confirms       []common.Hash   `json:"confirms"`
		FullSig        []byte          `json:"fullSig"`
		MinedTime      *uint64         `json:"minedTime"`
		HopCount       *uint64         `json:"hopCount"`
	}
	var header BlockHeader
	if err := json.Unmarshal(bh, &header); err != nil {
		return err
	}
	if header.ParentHash == nil {
		return errors.New("missing required field 'parentHash' for BlockHeader")
	}
	b.ParentHash = (*header.ParentHash).String()
	if header.Round == nil {
		return errors.New("missing required field 'round' for BlockHeader")
	}
	b.Round = *header.Round
	if header.Sig == nil {
		return errors.New("missing required field 'sig' for BlockHeader")
	}
	b.Sig = hexutil.Encode(header.Sig)
	if header.Coinbase == nil {
		return errors.New("missing required field 'miner' for BlockHeader")
	}
	b.Coinbase = header.Coinbase.String()
	if header.Difficulty == nil {
		return errors.New("missing required field 'difficulty' for BlockHeader")
	}
	b.Difficulty = header.Difficulty
	if header.Height == nil {
		return errors.New("missing required field 'height' for BlockHeader")
	}
	b.Height = *header.Height
	if header.Amount == nil {
		return errors.New("missing required field 'amount' for BlockHeader")
	}
	b.Amount = *header.Amount
	if header.GasLimit == nil {
		return errors.New("missing required field 'gasLimit' for BlockHeader")
	}
	b.GasLimit = *header.GasLimit
	if header.GasUsed == nil {
		return errors.New("missing required field 'gasUsed' for BlockHeader")
	}
	b.GasUsed = *header.GasUsed
	if header.StateHash == nil {
		return errors.New("missing required field 'stateHash' for BlockHeader")
	}
	b.StateHash = header.StateHash.String()
	if header.TxHash == nil {
		return errors.New("missing required field 'txHash' for BlockHeader")
	}
	b.TxHash = header.TxHash.String()
	if header.ReceiptHash == nil {
		return errors.New("missing required field 'receiptsRoot' for BlockHeader")
	}
	b.ReceiptHash = header.ReceiptHash.String()
	if header.ParentFullHash == nil {
		return errors.New("missing required field 'parentFullHash' for BlockHeader")
	}
	b.ParentFullHash = header.ParentFullHash.String()
	if header.Confirms != nil {
		b.Confirms = make([]string, len(header.Confirms))
		for i := 0; i < len(header.Confirms); i++ {
			b.Confirms[i] = header.Confirms[i].String()
		}
	}
	if header.FullSig == nil {
		return errors.New("missing required field 'fullSig' for BlockHeader")
	}
	b.FullSig = hexutil.Encode(header.FullSig)
	if header.MinedTime == nil {
		return errors.New("missing required field 'minedTime' for BlockHeader")
	}
	b.MinedTime = *header.MinedTime
	if header.HopCount == nil {
		return errors.New("missing required field 'hopCount' for BlockHeader")
	}
	b.HopCount = *header.HopCount
	type BlockBody struct {
		Transactions    []*Transaction `json:"transactions"`
		TxPackageHashes []common.Hash  `json:"txpackages"`
	}
	var body BlockBody
	if err := json.Unmarshal(bb, &body); err != nil {
		return err
	}
	if body.Transactions != nil {
		b.Transactions = body.Transactions
	}
	if body.TxPackageHashes != nil {
		b.TxPackageHashes = make([]string, len(body.TxPackageHashes))
		for i := 0; i < len(body.TxPackageHashes); i++ {
			b.TxPackageHashes[i] = body.TxPackageHashes[i].String()
		}
	}
	return nil
}

func (p *TxPackage) UnmarshalJSON(input []byte) error {
	type pkgData struct {
		Packer        *common.Address `json:"packer"`
		PackNonce     *hexutil.Uint64 `json:"nonce"`
		Transactions  []*Transaction  `json:"transactions"`
		BlockFullHash *common.Hash    `json:"blockFullHash"`
		R             *hexutil.Big    `json:"r"`
		S             *hexutil.Big    `json:"s"`
		V             *hexutil.Big    `json:"v"`
		Hash          *common.Hash    `json:"hash"`
		GenTime       *uint64         `json:"genTime"`
		HopCount      *uint64         `json:"hopCount"`
	}
	var dec pkgData
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Packer != nil {
		p.Packer = dec.Packer.String()
	}
	if dec.PackNonce != nil {
		p.PackNonce = uint64(*dec.PackNonce)
	}
	if dec.Transactions != nil {
		p.Transactions = dec.Transactions
	}
	if dec.BlockFullHash != nil {
		p.BlockFullHash = dec.BlockFullHash.String()
	}
	if dec.R != nil {
		p.R = (*big.Int)(dec.R)
	}
	if dec.S != nil {
		p.S = (*big.Int)(dec.S)
	}
	if dec.V != nil {
		p.V = (*big.Int)(dec.V)
	}
	if dec.Hash != nil {
		p.Hash = dec.Hash.String()
	}
	if dec.GenTime != nil {
		p.GenTime = *dec.GenTime
	}
	return nil
}

func (t *Transaction) UnmarshalJSON(input []byte) error {
	type txdata struct {
		AccountNonce *hexutil.Uint64 `json:"nonce"`
		Price        *hexutil.Big    `json:"gasPrice"`
		GasLimit     *hexutil.Uint64 `json:"gas"`
		Recipient    *common.Address `json:"to"`
		Amount       *hexutil.Big    `json:"value"`
		Payload      *hexutil.Bytes  `json:"input"`
		Broadcast    *bool           `json:"broadcast"`
		V            *hexutil.Big    `json:"v"`
		R            *hexutil.Big    `json:"r"`
		S            *hexutil.Big    `json:"s"`
		Hash         *common.Hash    `json:"hash"`
	}

	var dec txdata
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.AccountNonce == nil {
		return errors.New("missing required field 'nonce' for txdata")
	}
	t.AccountNonce = uint64(*dec.AccountNonce)
	if dec.Price == nil {
		return errors.New("missing required field 'gasPrice' for txdata")
	}
	t.Price = (*big.Int)(dec.Price)
	if dec.GasLimit == nil {
		return errors.New("missing required field 'gas' for txdata")
	}
	t.GasLimit = uint64(*dec.GasLimit)
	if dec.Recipient != nil {
		t.Recipient = dec.Recipient.String()
	}
	if dec.Amount == nil {
		return errors.New("missing required field 'value' for txdata")
	}
	t.Amount = (*big.Int)(dec.Amount)
	if dec.Payload == nil {
		return errors.New("missing required field 'input' for txdata")
	}
	t.Payload = *dec.Payload
	if dec.Broadcast == nil {
		return errors.New("missing required field 'broadcast' for txdata")
	}
	t.Broadcast = *dec.Broadcast
	if dec.V == nil {
		return errors.New("missing required field 'v' for txdata")
	}
	t.V = (*big.Int)(dec.V)
	if dec.R == nil {
		return errors.New("missing required field 'r' for txdata")
	}
	t.R = (*big.Int)(dec.R)
	if dec.S == nil {
		return errors.New("missing required field 's' for txdata")
	}
	t.S = (*big.Int)(dec.S)
	if dec.Hash != nil {
		t.Hash = dec.Hash.String()
	}
	return nil
}

func (l *Log) UnmarshalJSON(input []byte) error {
	type Log struct {
		Address     *common.Address `json:"address"`
		Topics      []common.Hash   `json:"topics"`
		Data        *hexutil.Bytes  `json:"data"`
		BlockNumber *hexutil.Uint64 `json:"blockNumber"`
		TxHash      *common.Hash    `json:"transactionHash"`
		PkgIndex    *uint32         `json:"packageIndex"`
		TxIndex     *uint32         `json:"transactionIndex"`
		Index       *uint32         `json:"logIndex"`
	}
	var dec Log
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Address == nil {
		return errors.New("missing required field 'address' for Log")
	}
	l.Address = dec.Address.String()
	if dec.Topics == nil {
		return errors.New("missing required field 'topics' for Log")
	}
	l.Topics = make([]string, len(dec.Topics))
	for i := 0; i < len(dec.Topics); i++ {
		l.Topics[i] = dec.Topics[i].String()
	}
	if dec.Data == nil {
		return errors.New("missing required field 'data' for Log")
	}
	l.Data = *dec.Data
	if dec.BlockNumber == nil {
		return errors.New("missing required field 'blockNumber' for Log")
	}
	l.BlockNumber = uint64(*dec.BlockNumber)
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionHash' for Log")
	}
	l.TxHash = dec.TxHash.String()
	if dec.PkgIndex == nil {
		return errors.New("missing required field 'packageIndex' for Log")
	}
	l.PkgIndex = *dec.PkgIndex
	if dec.TxIndex == nil {
		return errors.New("missing required field 'transactionIndex' for Log")
	}
	l.TxIndex = *dec.TxIndex
	if dec.Index == nil {
		return errors.New("missing required field 'logIndex' for Log")
	}
	l.Index = *dec.Index
	return nil
}

func (r *Receipt) UnmarshalJSON(input []byte) error {
	type Receipt struct {
		PostState         *hexutil.Bytes  `json:"root"`
		Status            *hexutil.Uint64 `json:"status"`
		CumulativeGasUsed *hexutil.Uint64 `json:"cumulativeGasUsed"`
		Bloom             *types.Bloom    `json:"logsBloom"`
		Logs              []*Log          `json:"logs"`
		TxHash            *common.Hash    `json:"transactionHash"`
		ContractAddress   *common.Address `json:"contractAddress"`
		GasUsed           *hexutil.Uint64 `json:"gasUsed"`
	}
	var dec Receipt
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.PostState != nil {
		r.PostState = *dec.PostState
	}
	if dec.Status != nil {
		r.Status = uint64(*dec.Status)
	}
	if dec.CumulativeGasUsed == nil {
		return errors.New("missing required field 'cumulativeGasUsed' for Receipt")
	}
	r.CumulativeGasUsed = uint64(*dec.CumulativeGasUsed)
	if dec.Bloom == nil {
		return errors.New("missing required field 'logsBloom' for Receipt")
	}
	bloom := [4096]byte{}
	copy(bloom[:], dec.Bloom[:])
	r.Bloom = &bloom
	if dec.Logs == nil {
		return errors.New("missing required field 'logs' for Receipt")
	}
	r.Logs = dec.Logs
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionHash' for Receipt")
	}
	r.TxHash = dec.TxHash.String()
	if dec.ContractAddress != nil {
		r.ContractAddress = dec.ContractAddress.String()
	}
	if dec.GasUsed == nil {
		return errors.New("missing required field 'gasUsed' for Receipt")
	}
	r.GasUsed = uint64(*dec.GasUsed)
	return nil
}

func (t *TransactionDetails) UnmarshalJSON(input []byte) error {
	type RPCTransaction struct {
		From      *common.Address `json:"from"`
		Hash      *common.Hash    `json:"hash"`
		Nonce     *hexutil.Uint64 `json:"nonce"`
		To        *common.Address `json:"to"`
		Value     *hexutil.Big    `json:"value"`
		Price     *hexutil.Big    `json:"gasPrice"`
		GasLimit  *hexutil.Uint64 `json:"gas"`
		Payload   *hexutil.Bytes  `json:"input"`
		Broadcast *bool           `json:"broadcast"`
		V         *hexutil.Big    `json:"v"`
		R         *hexutil.Big    `json:"r"`
		S         *hexutil.Big    `json:"s"`
		BlockHash *common.Hash    `json:"blockHash"`
		Receipt   *Receipt        `json:"receipt"`
	}
	var dec RPCTransaction
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.From != nil {
		t.From = dec.From.String()
	}
	if dec.Hash != nil {
		t.Hash = dec.Hash.String()
	}
	if dec.Nonce != nil {
		t.Nonce = uint64(*dec.Nonce)
	}
	if dec.To != nil {
		t.To = dec.To.String()
	}
	if dec.Value != nil {
		t.Value = (*big.Int)(dec.Value)
	}
	if dec.Price != nil {
		t.Price = (*big.Int)(dec.Price)
	}
	if dec.GasLimit != nil {
		t.GasLimit = uint64(*dec.GasLimit)
	}
	if dec.Payload != nil {
		t.Payload = *dec.Payload
	}
	if dec.Broadcast != nil {
		t.Broadcast = *dec.Broadcast
	}
	if dec.V != nil {
		t.V = (*big.Int)(dec.V)
	}
	if dec.R != nil {
		t.R = (*big.Int)(dec.R)
	}
	if dec.S != nil {
		t.S = (*big.Int)(dec.S)
	}
	if dec.BlockHash != nil {
		t.BlockHash = dec.BlockHash.String()
	}
	if dec.Receipt != nil {
		t.Receipt = dec.Receipt
	}
	return nil
}

func (c *CallResult) UnmarshalJSON(input []byte) error {
	type CallResult struct {
		Logs    []*Log
		GasUsed *hexutil.Uint64
	}
	var dec CallResult
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Logs != nil {
		c.Logs = dec.Logs
	}
	if dec.GasUsed != nil {
		c.GasUsed = uint64(*dec.GasUsed)
	}
	return nil
}
