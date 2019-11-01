package abi

import (
	"bytes"
	"testing"

	"github.com/fractal-platform/fractal/common"
	. "github.com/smartystreets/goconvey/convey"
)

func TestVaruint32(t *testing.T) {
	Convey("pack/unpack varuint32", t, func() {
		buf := make([]byte, 0)
		rw := bytes.NewBuffer(buf)
		Convey("value is zero", func() {
			v := uint32(0)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("00"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 0)
		})
		Convey("value is small", func() {
			v := uint32(100)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("64"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 100)
		})
		Convey("value is 128", func() {
			v := uint32(128)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("8001"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 128)
		})
		Convey("value is in (128, 16384)", func() {
			v := uint32(300)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("AC02"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 300)
		})
		Convey("value is 16384", func() {
			v := uint32(16384)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("808001"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 16384)
		})
		Convey("value is in (16384, 2097152)(1)", func() {
			v := uint32(30000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("B0EA01"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 30000)
		})
		Convey("value is in (16384, 2097152)(2)", func() {
			v := uint32(300000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("E0A712"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 300000)
		})
		Convey("value is in (2097152, 268435456)(1)", func() {
			v := uint32(3000000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("C08DB701"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 3000000)
		})
		Convey("value is in (2097152, 268435456)(2)", func() {
			v := uint32(30000000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("8087A70E"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 30000000)
		})
		Convey("value is in (268435456, 4294967295)(1)", func() {
			v := uint32(300000000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("80C6868F01"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 300000000)
		})
		Convey("value is in (268435456, 4294967295)(2)", func() {
			v := uint32(3000000000)
			err := packVaruint32(v, rw)
			So(err, ShouldBeNil)
			bs := rw.Bytes()
			So(bs, ShouldResemble, common.Hex2Bytes("80BCC1960B"))

			var r uint32
			err = unpackVaruint32(&r, rw)
			So(err, ShouldBeNil)
			So(r, ShouldEqual, 3000000000)
		})
	})
}
