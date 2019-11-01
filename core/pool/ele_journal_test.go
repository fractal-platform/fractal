package pool

import (
	"os"
	"testing"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/stretchr/testify/assert"
)

func TestInsertAndLoad(t *testing.T) {
	log.SetDefaultLogger(log.InitLog15Logger(log.LvlDebug, os.Stdout))
	journal := newEleJournal("tests.ele.rlp", TxPackageType)
	defer os.Remove("tests.ele.rlp")
	defer journal.close()

	err := journal.load(func(elements []Element) []error {
		assert.Empty(t, elements, "at first journal should be empty.")
		return nil
	})
	assert.Nil(t, err, "there should be no error")

	p1 := types.NewTxPackage(common.Address{}, 1, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	p2 := types.NewTxPackage(common.Address{}, 2, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	p3 := types.NewTxPackage(common.Address{}, 3, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	journal.insert(p1)
	journal.insert(p2)
	journal.insert(p3)
	err = journal.load(func(elements []Element) []error {
		s := []Element{p1, p2, p3}
		assert.EqualValues(t, s, elements, "the slice value should be same.")
		return nil
	})
	assert.Nil(t, err, "there should be no error")
}

func TestEleJournal_Rotate(t *testing.T) {
	log.SetDefaultLogger(log.InitLog15Logger(log.LvlDebug, os.Stdout))
	journal := newEleJournal("tests.ele.rlp", TxPackageType)
	defer os.Remove("tests.ele.rlp")
	defer journal.close()

	p1 := types.NewTxPackage(common.Address{}, 1, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	p2 := types.NewTxPackage(common.Address{}, 2, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	p3 := types.NewTxPackage(common.Address{}, 3, nil, common.Hash{}, uint64(time.Now().UnixNano()))
	journal.insert(p1)
	journal.insert(p2)
	journal.insert(p3)
	err := journal.load(func(elements []Element) []error {
		s := []Element{p1, p2, p3}
		assert.EqualValues(t, s, elements, "the slice value should be same.")
		return nil
	})
	assert.Nil(t, err, "there should be no error")

	elems := make(map[common.Address][]Element)
	elems[common.Address{}] = []Element{p1, p2}
	journal.rotate(elems)
	err = journal.load(func(elements []Element) []error {
		assert.Equal(t, uint64(1), elements[0].Nonce(), "first one should be 1")
		assert.Equal(t, uint64(2), elements[1].Nonce(), "second one should be 2")
		return nil
	})
	assert.Nil(t, err, "there should be no error")
}
