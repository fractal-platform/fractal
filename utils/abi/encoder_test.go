package abi

import (
	"bytes"
	"testing"

	"github.com/fractal-platform/fractal/common"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBool(t *testing.T) {
	Convey("encode/decode bool", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is true", func() {
			v := true
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("01"))

			var r bool
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldBeTrue)
		})
		Convey("value is false", func() {
			v := false
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("00"))

			var r bool
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldBeFalse)
		})
	})
}

func TestInt8(t *testing.T) {
	Convey("encode/decode int8", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is positive", func() {
			v := int8(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("64"))

			var r int8
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is negative", func() {
			v := int8(-100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9C"))

			var r int8
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, -100)
		})
	})
}

func TestUint8(t *testing.T) {
	Convey("encode/decode uint8", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is small", func() {
			v := uint8(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("64"))

			var r uint8
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is big", func() {
			v := uint8(200)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("C8"))

			var r uint8
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 200)
		})
	})
}

func TestInt16(t *testing.T) {
	Convey("encode/decode int16", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is positive", func() {
			v := int16(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("6400"))

			var r int16
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is negative", func() {
			v := int16(-100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9CFF"))

			var r int16
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, -100)
		})
	})
}

func TestUint16(t *testing.T) {
	Convey("encode/decode uint16", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is small", func() {
			v := uint16(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("6400"))

			var r uint16
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is big", func() {
			v := uint16(200)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("C800"))

			var r uint16
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 200)
		})
	})
}

func TestInt32(t *testing.T) {
	Convey("encode/decode int32", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is positive", func() {
			v := int32(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("64000000"))

			var r int32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is negative", func() {
			v := int32(-100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9CFFFFFF"))

			var r int32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, -100)
		})
	})
}

func TestUint32(t *testing.T) {
	Convey("encode/decode uint32", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is small", func() {
			v := uint32(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("64000000"))

			var r uint32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is big", func() {
			v := uint32(200)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("C8000000"))

			var r uint32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 200)
		})
	})
}

func TestInt64(t *testing.T) {
	Convey("encode/decode int64", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is positive", func() {
			v := int64(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("6400000000000000"))

			var r int64
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is negative", func() {
			v := int64(-100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9CFFFFFFFFFFFFFF"))

			var r int64
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, -100)
		})
	})
}

func TestUint64(t *testing.T) {
	Convey("encode/decode uint64", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is small", func() {
			v := uint64(100)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("6400000000000000"))

			var r uint64
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is big", func() {
			v := uint64(200)
			err := Encode(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("C800000000000000"))

			var r uint64
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 200)
		})
	})
}

func TestString(t *testing.T) {
	Convey("encode/decode string", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)

		s := "hello"
		err := Encode(s, rw)
		So(err, ShouldBeNil)
		bs := rw.Bytes()
		So(bs, ShouldResemble, common.Hex2Bytes("0568656C6C6F"))

		var r string
		err = Decode(&r, rw)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "hello")
	})
}

func TestAddress(t *testing.T) {
	Convey("encode/decode address", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)

		s := common.HexToAddress("8724e3e81395f8ab759ffcd97c2a70985416e24c")
		err := Encode(s, rw)
		So(err, ShouldBeNil)
		bs := rw.Bytes()
		So(bs, ShouldResemble, common.Hex2Bytes("8724e3e81395f8ab759ffcd97c2a70985416e24c"))

		var r common.Address
		err = Decode(&r, rw)
		So(err, ShouldBeNil)
		So(r[:], ShouldResemble, common.Hex2Bytes("8724e3e81395f8ab759ffcd97c2a70985416e24c"))
	})
}

func TestHash(t *testing.T) {
	Convey("encode/decode hash", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)

		s := common.HexToHash("44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7")
		err := Encode(s, rw)
		So(err, ShouldBeNil)
		bs := rw.Bytes()
		So(bs, ShouldResemble, common.Hex2Bytes("44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7"))

		var r common.Hash
		err = Decode(&r, rw)
		So(err, ShouldBeNil)
		So(r[:], ShouldResemble, common.Hex2Bytes("44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7"))
	})
}

func TestStruct(t *testing.T) {
	Convey("encode/decode struct", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("simple struct", func() {
			type __ts struct {
				A int8
				B uint8
				C int16
				D uint16
				E int32
				F uint32
				G int64
				H uint64
				I string
				J bool
			}
			var s = __ts{-100, 100, -100, 100, -100, 100, -100, 100, "hello", true}
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9C649CFF64009CFFFFFF640000009CFFFFFFFFFFFFFF64000000000000000568656C6C6F01"))

			var r __ts
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r.A, ShouldEqual, -100)
			So(r.B, ShouldEqual, 100)
			So(r.C, ShouldEqual, -100)
			So(r.D, ShouldEqual, 100)
			So(r.E, ShouldEqual, -100)
			So(r.F, ShouldEqual, 100)
			So(r.G, ShouldEqual, -100)
			So(r.H, ShouldEqual, 100)
			So(r.I, ShouldEqual, "hello")
			So(r.J, ShouldBeTrue)
		})
		Convey("complex struct", func() {
			type __ts2 struct {
				A uint32
				B bool
			}
			type __ts struct {
				A int8
				B string
				C __ts2
			}
			var s = __ts{-100, "hello", __ts2{100, true}}
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9C0568656C6C6F6400000001"))

			var r __ts
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r.A, ShouldEqual, -100)
			So(r.B, ShouldEqual, "hello")
			So(r.C.A, ShouldEqual, 100)
			So(r.C.B, ShouldBeTrue)
		})
		Convey("struct with hash", func() {
			type __ts struct {
				A int8
				B string
				C common.Hash
			}
			var s = __ts{-100, "hello", common.HexToHash("44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7")}
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("9C0568656C6C6F44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7"))

			var r __ts
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r.A, ShouldEqual, -100)
			So(r.B, ShouldEqual, "hello")
			So(r.C[:], ShouldResemble, common.Hex2Bytes("44f7ff9a8b9b34e2f06996e9d97765229a6d577440acc413c9ebf5237cff2cb7"))
		})
	})
}

func TestSlice(t *testing.T) {
	Convey("encode/decode slice", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("simple slice", func() {
			var s = make([]int32, 3)
			s[0] = 100
			s[1] = 100
			s[2] = 100
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("03640000006400000064000000"))

			var r []int32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(len(r), ShouldEqual, 3)
			So(r[0], ShouldEqual, 100)
			So(r[1], ShouldEqual, 100)
			So(r[2], ShouldEqual, 100)
		})
		Convey("complex slice", func() {
			var s = make([][]int32, 2)
			s[0] = make([]int32, 2)
			s[1] = make([]int32, 2)
			s[0][0] = 100
			s[0][1] = 100
			s[1][0] = 100
			s[1][1] = 100
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("02026400000064000000026400000064000000"))

			var r [][]int32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(len(r), ShouldEqual, 2)
			So(len(r[0]), ShouldEqual, 2)
			So(len(r[1]), ShouldEqual, 2)
			So(r[0][0], ShouldEqual, 100)
			So(r[0][1], ShouldEqual, 100)
			So(r[1][0], ShouldEqual, 100)
			So(r[1][1], ShouldEqual, 100)
		})
	})
}

func TestStructAndSlice(t *testing.T) {
	Convey("encode/decode struct&slice", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("slice of struct", func() {
			type __ts struct {
				A uint32
				B bool
			}
			var s = make([]__ts, 2)
			s[0].A = 100
			s[0].B = true
			s[1].A = 100
			s[1].B = false
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("0264000000016400000000"))

			var r []__ts
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(len(r), ShouldEqual, 2)
			So(r[0].A, ShouldEqual, 100)
			So(r[0].B, ShouldBeTrue)
			So(r[1].A, ShouldEqual, 100)
			So(r[1].B, ShouldBeFalse)
		})
		Convey("slice in struct", func() {
			type __ts struct {
				A uint32
				B bool
				C []uint8
			}
			var s __ts
			s.A = 100
			s.B = true
			s.C = make([]uint8, 2)
			s.C[0] = 100
			s.C[1] = 100
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("6400000001026464"))

			var r __ts
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(r.A, ShouldEqual, 100)
			So(r.B, ShouldBeTrue)
			So(len(r.C), ShouldEqual, 2)
			So(r.C[0], ShouldEqual, 100)
			So(r.C[1], ShouldEqual, 100)
		})
	})
}

func TestArray(t *testing.T) {
	Convey("encode/decode array", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("simple array", func() {
			var s [3]uint32
			s[0] = 100
			s[1] = 100
			s[2] = 100
			err := Encode(s, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("03640000006400000064000000"))

			var r [3]int32
			err = Decode(&r, rw)
			So(err, ShouldBeNil)
			So(len(r), ShouldEqual, 3)
			So(r[0], ShouldEqual, 100)
			So(r[1], ShouldEqual, 100)
			So(r[2], ShouldEqual, 100)
		})
	})
}
