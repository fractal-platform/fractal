package state

import (
	"bytes"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/trie"
)

// NewStateSync create a new state trie download scheduler.
func NewStateSync(root common.Hash, database dbwrapper.Database) *trie.Sync {
	var syncer *trie.Sync
	callback := func(leaf []byte, parent common.Hash) error {
		var obj Account
		if err := rlp.Decode(bytes.NewReader(leaf), &obj); err != nil {
			return err
		}
		syncer.AddSubTrie(obj.Root, 64, parent, nil)
		syncer.AddRawEntry(common.BytesToHash(obj.CodeHash), 64, parent)
		return nil
	}
	syncer = trie.NewSync(root, database, callback)
	return syncer
}
