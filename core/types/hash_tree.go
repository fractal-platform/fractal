package types

import (
	"encoding/json"
	"errors"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/golang-collections/collections/stack"
)

const (
	MarkTreePoint     = ^uint64(0)
	HashTreeMinLength = 20
)

var (
	errTreeElemOutOfBound = errors.New("tree elem out of bound")
	errTreeNotComplete    = errors.New("hash tree not complete")
	errTreeMainListWrong  = errors.New("hash tree main chain list is wrong")
)

type TreeElem struct {
	FullHash       common.Hash
	Confirms       []uint64
	ParentFullHash uint64
}

func (t TreeElem) String() string {
	bytes, _ := json.Marshal(t)
	return string(bytes)
}

type TreeElems []*TreeElem

type HashTree struct {
	//cache
	mainChainList []uint64

	RootIndex uint64
	Elems     TreeElems
}

func (h *HashTree) getTreeElem(index uint64) (*TreeElem, error) {
	//log.Info("hash tree", "index", index, "treeLength", len(h.Elems))
	if index >= uint64(len(h.Elems)) {
		return nil, errTreeElemOutOfBound
	}
	return h.Elems[index], nil
}

// Verify HashTree's
func (h *HashTree) CalcAccHash(verifiedAccHashMap map[common.Hash]common.Hash) (common.Hash, error) {
	//build main chain list in hashTree
	err := h.buildMainChainList()
	if err != nil {
		return common.Hash{}, err
	}

	//use dfs to verify
	for _, hashIndex := range h.mainChainList {
		err = h.dfsVerifyTree(verifiedAccHashMap, hashIndex)
		if err != nil {
			return common.Hash{}, err
		}
	}

	//calc accHash
	rootTreeElem, _ := h.getTreeElem(h.RootIndex)
	rootAccHash, ok := verifiedAccHashMap[rootTreeElem.FullHash]
	if !ok {
		return common.Hash{}, errTreeNotComplete
	}
	return rootAccHash, nil
}

func (h *HashTree) getParentAndConfirms(hashIndex uint64) ([]uint64, error) {
	var res []uint64
	treeElem, err := h.getTreeElem(hashIndex)
	if err != nil {
		return nil, err
	}

	if treeElem.ParentFullHash == MarkTreePoint {
		return nil, nil
	}

	_, err = h.getTreeElem(treeElem.ParentFullHash)
	if err != nil {
		return nil, err
	}

	var confirms []uint64

	for _, confirmIndex := range treeElem.Confirms {
		_, err := h.getTreeElem(confirmIndex)
		if err != nil {
			return nil, err
		}
		confirms = append(confirms, confirmIndex)
	}

	res = append(confirms, treeElem.ParentFullHash)

	return res, nil
}

func (h *HashTree) buildMainChainList() error {
	treeElem, err := h.getTreeElem(h.RootIndex)
	if err != nil {
		return err
	}
	h.mainChainList = append(h.mainChainList, h.RootIndex)
	for treeElem.ParentFullHash != MarkTreePoint {
		currentIndex := treeElem.ParentFullHash
		treeElem, err = h.getTreeElem(currentIndex)
		if err != nil {
			return err
		}
		h.mainChainList = append([]uint64{currentIndex}, h.mainChainList...)
	}
	//cut head because tree point is not in HashTree
	h.mainChainList = h.mainChainList[1:]
	log.Info("build main chain list", "list", h.mainChainList, "len(list)", len(h.mainChainList), "rootIndex", h.RootIndex)
	return nil
}

func (h *HashTree) parentAndConfirmsUnverified(verifiedHashes map[common.Hash]common.Hash, hashIndex uint64) ([]uint64, error) {
	var unverified []uint64
	//verify
	parentAndConfirms, err := h.getParentAndConfirms(hashIndex)
	if err != nil {
		return nil, err
	}
	for _, index := range parentAndConfirms {
		treeElem, _ := h.getTreeElem(index)

		if _, ok := verifiedHashes[treeElem.FullHash]; !ok {
			unverified = append(unverified, index)
		}
	}
	return unverified, nil
}

//assume its parent and confirms is verified already
func (h *HashTree) calcuAccIndexHash(verifiedHashes map[common.Hash]common.Hash, hashIndex uint64) common.Hash {
	parentAndConfirms, _ := h.getParentAndConfirms(hashIndex)

	var accHashes []common.Hash
	for _, index := range parentAndConfirms {
		treeElem, _ := h.getTreeElem(index)
		accHash := verifiedHashes[treeElem.FullHash]
		accHashes = append(accHashes, accHash)
	}
	self, _ := h.getTreeElem(hashIndex)
	accHashes = append(accHashes, self.FullHash)

	return common.RlpHash(accHashes)
}

func (h *HashTree) dfsVerifyTree(verifiedHashes map[common.Hash]common.Hash, index uint64) error {
	stack := stack.New()

	stack.Push(index)
	for stack.Len() > 0 {
		elemIndex := stack.Peek().(uint64)

		treeElem, err := h.getTreeElem(elemIndex)
		if err != nil {
			return err
		}

		//already verified
		if _, ok := verifiedHashes[treeElem.FullHash]; ok {
			stack.Pop()
			continue
		}

		unverifiedTreeHashIndexes, err := h.parentAndConfirmsUnverified(verifiedHashes, elemIndex)
		if err != nil {
			return err
		}
		//parent and confirms
		if len(unverifiedTreeHashIndexes) == 0 {
			accHash := h.calcuAccIndexHash(verifiedHashes, elemIndex)
			verifiedHashes[treeElem.FullHash] = accHash
			stack.Pop()
			continue
		} else {
			for _, unverifiedHashIndex := range unverifiedTreeHashIndexes {
				stack.Push(unverifiedHashIndex)
			}
		}
	}
	return nil
}

func (h *HashTree) PostOrderTraversal(index uint64, addedSet mapset.Set) []common.Hash {
	//log.Info("hash tree post order traversal", "index", index, "len(Added)", addedSet.Cardinality())

	var result []common.Hash
	treeElem, err := h.getTreeElem(index)
	if err != nil {
		return result
	}

	if addedSet.Contains(index) {
		return result
	}

	for _, confirmIndex := range treeElem.Confirms {
		if addedSet.Contains(confirmIndex) {
			continue
		}
		confirmRes := h.PostOrderTraversal(confirmIndex, addedSet)
		result = append(result, confirmRes...)
	}
	if !addedSet.Contains(treeElem.ParentFullHash) {
		parentRes := h.PostOrderTraversal(treeElem.ParentFullHash, addedSet)
		result = append(result, parentRes...)
	}

	result = append(result, treeElem.FullHash)
	addedSet.Add(index)
	//log.Info("hash tree post order traversal", "index", index, "len(Added)", addedSet.Cardinality())

	return result
}

func (h *HashTree) RetrieveMainChainSet() (mapset.Set, error) {
	resSet := mapset.NewSet()
	treeElem, _ := h.getTreeElem(h.RootIndex)
	resSet.Add(treeElem.FullHash)

	for treeElem.ParentFullHash != MarkTreePoint {
		tempTreeElem, err := h.getTreeElem(treeElem.ParentFullHash)
		if err != nil {
			return nil, errTreeMainListWrong
		}
		resSet.Add(treeElem.FullHash)
		treeElem = tempTreeElem
	}

	return resSet, nil
}
