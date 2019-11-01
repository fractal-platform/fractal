package types

import (
	"encoding/json"
	"io"
	"strconv"
	"sync/atomic"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/rlp"
)

type TreePoint struct {
	Height            uint64
	FullHash          common.Hash
	MainChainHashList []common.Hash
	HashPairs         []HashPairFullAcc // fullHash -> accHash
}

func (t *TreePoint) UnRepeatedHashes() []common.Hash {
	var hashes []common.Hash
	hashesMap := mapset.NewSet()
	for _, hash := range t.MainChainHashList {
		if !hashesMap.Contains(hash) {
			hashesMap.Add(hash)
			hashes = append(hashes, hash)
		}
	}

	for key := range t.RetrieveAccHashMap() {
		if !hashesMap.Contains(key) {
			hashesMap.Add(key)
			hashes = append(hashes, key)
		}

	}
	return hashes
}

func (t TreePoint) String() string {
	var res string
	res = res + "{fullHash:" + t.FullHash.String() + " Height:" + strconv.FormatUint(t.Height, 10) + " MainChainHashList:["
	for _, confirm := range t.MainChainHashList {
		res = res + confirm.String() + ","
	}
	res = res + "] HashPair:["
	for _, hashPair := range t.HashPairs {
		res = res + hashPair.String()
	}
	res = res + "]}"
	return res
}

type HashPairFullAcc [2]common.Hash

func (h HashPairFullAcc) String() string {
	return "{" + h[0].String() + "--->" + h[1].String() + "}"
}

func (t *TreePoint) RetrieveAccHashMap() map[common.Hash]common.Hash {
	res := make(map[common.Hash]common.Hash)
	for _, hashPair := range t.HashPairs {
		res[hashPair[0]] = hashPair[1]
	}
	return res
}

type CheckPoint struct {
	*TreePoint

	// cache
	treePointHash atomic.Value
}

func (c *CheckPoint) Hash() common.Hash {
	if hash := c.treePointHash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := common.RlpHash([]interface{}{
		c.TreePoint,
	})
	c.treePointHash.Store(v)
	return v
}

func (c *CheckPoint) DecodeRLP(s *rlp.Stream) error {
	var content TreePoint
	if err := s.Decode(&content); err != nil {
		return err
	}
	c.TreePoint = &content
	return nil
}

func (c *CheckPoint) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, *c.TreePoint)
}

func (c *CheckPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.TreePoint)
}

func (c *CheckPoint) UnMarshalJSON(input []byte) error {
	var treePoint TreePoint
	if err := json.Unmarshal(input, &treePoint); err != nil {
		return err
	}
	c.TreePoint = &treePoint
	return nil
}

type SignedCheckPoint struct {
	CheckPoint *CheckPoint
	Sign       []byte
}

type SignedCheckPointHash struct {
	Hash common.Hash
	Sign []byte
}

type CheckPointNodeTypeEnum byte

const (
	NormalNode CheckPointNodeTypeEnum = iota
	SpecialNode
)

var (
	CheckPointNodeRPC       = "http://127.0.0.1:8545"
	CheckPointNodePubKeyStr = "0x04682a506e727bff64e6e5ce526dcd51a4b5d660b87b00caa8f95f29ac2c32d5690efdaa793152ced675d83cbdbd262461cfe4609c476966ca2f16882eb3d83fe8"
)
