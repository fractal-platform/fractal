package nonces

import (
	"fmt"
	"testing"

	"github.com/fractal-platform/fractal/common/hexutil"
)

func (s *NonceSet) Print() {
	fmt.Println("Start:", s.Start)
	fmt.Println("BitMask:", hexutil.Encode(s.BitMask))
	fmt.Println("Length:", s.Length)
	fmt.Println()
}

func TestNonceSet(t *testing.T) {
	nonceSet := &NonceSet{
		Start:   111,
		BitMask: NonceBitMask{162, 68, 32, 34}, // 10100010, 01000100, 00100000, 00100010
		Length:  31,
	}

	nonceSet.Print()

	nonceSet.Add(112, 1024) // 11100010, 01000100, 00100000, 00100010

	nonceSet.Print()

	nonceSet.Add(142, 1024) // 11100010, 01000100, 00100000, 00100011

	nonceSet.Print()

	nonceSet.Add(143, 1024) // 11100010, 01000100, 00100000, 00100011, 10000000

	nonceSet.Print()

	nonceSet.Reset(112) // 11000100, 10001000, 01000000, 01000111
	nonceSet.Print()

	nonceSet.Reset(129) // 10000000, 10001110
	nonceSet.Print()
}
