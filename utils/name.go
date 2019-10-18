package utils

import (
	"errors"
	"math"
)

func String2Uint64(str string) (uint64, error) {
	var value uint64 = 0

	if len(str) > 13 {
		return 0, errors.New("string is too long to be a valid name")
	}

	if len(str) == 0 {
		return 0, nil
	}

	var n = int(math.Min(float64(len(str)), 12.0))

	for i := 0; i < n; i++ {
		value <<= 5
		v, err := Char2Value(str[i])
		if err != nil {
			return 0, err
		}
		value |= uint64(v)
	}
	value <<= uint(4 + 5*(12-n))
	if len(str) == 13 {
		v, err := Char2Value(str[12])
		if err != nil {
			return 0, err
		}
		if v > 0x0F {
			return 0, errors.New("thirteenth character in name cannot be a letter that comes after j")
		}
		value |= uint64(v)
	}
	return value, nil
}

func Char2Value(c uint8) (uint8, error) {
	if c == '.' {
		return 0, nil
	} else if c >= '1' && c <= '5' {
		return (c - '1') + 1, nil
	} else if c >= 'a' && c <= 'z' {
		return (c - 'a') + 6, nil
	}

	return 0, errors.New("character is not in allowed character set for names")
}

func Uint642String(c uint64) string {
	const charmap = ".12345abcdefghijklmnopqrstuvwxyz"
	var mask uint64 = 0xF800000000000000
	var str []uint8
	var i int

	for i = 0; i < 13; i++ {
		if c == 0 {
			return string(str)
		}

		if i == 12 {
			index := (c & mask) >> 60
			str = append(str, charmap[index])
		} else {
			index := (c & mask) >> 59
			str = append(str, charmap[index])
		}

		c <<= 5
	}
	return string(str)

}
