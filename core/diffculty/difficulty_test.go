// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package difficulty contains the implementation of difficulty change for fractal.
package difficulty

import (
	"encoding/json"
	"github.com/fractal-platform/fractal/common/math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

type diffTest struct {
	ParentTimestamp   uint64
	ParentDifficulty  *big.Int
	CurrentTimestamp  uint64
	CurrentDifficulty *big.Int
}

func (d *diffTest) UnmarshalJSON(b []byte) (err error) {
	var ext struct {
		ParentTimestamp   string
		ParentDifficulty  string
		CurrentTimestamp  string
		CurrentDifficulty string
	}
	if err := json.Unmarshal(b, &ext); err != nil {
		return err
	}

	d.ParentTimestamp = math.MustParseUint64(ext.ParentTimestamp)
	d.ParentDifficulty = math.MustParseBig256(ext.ParentDifficulty)
	d.CurrentTimestamp = math.MustParseUint64(ext.CurrentTimestamp)
	d.CurrentDifficulty = math.MustParseBig256(ext.CurrentDifficulty)

	return nil
}

func TestCalcDifficulty(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "tests", "testdata", "BasicTests", "difficulty.json"))
	if err != nil {
		t.Skip(err)
	}
	defer file.Close()

	tests := make(map[string]diffTest)
	err = json.NewDecoder(file).Decode(&tests)
	if err != nil {
		t.Fatal(err)
	}

	for name, test := range tests {
		diff := CalcDifficulty(test.CurrentTimestamp, test.ParentTimestamp, test.ParentDifficulty)
		if diff.Cmp(test.CurrentDifficulty) != 0 {
			t.Error(name, "failed. Expected", test.CurrentDifficulty, "and calculated", diff)
		}
	}
}
