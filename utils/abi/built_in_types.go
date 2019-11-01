package abi

import (
	"encoding/binary"
	"io"
	"reflect"

	"github.com/fractal-platform/fractal/common"
)

func packBool(value interface{}, writer io.Writer) error {
	var err error
	v := value.(bool)
	if v {
		_, err = writer.Write([]byte{1})
	} else {
		_, err = writer.Write([]byte{0})
	}
	return err
}

func unpackBool(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 1)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	if bytes[0] == 0 {
		v.SetBool(false)
	} else {
		v.SetBool(true)
	}
	return nil
}

func packInt8(value interface{}, writer io.Writer) error {
	var err error
	v := byte(value.(int8))
	_, err = writer.Write([]byte{v})
	return err
}

func unpackInt8(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 1)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetInt(int64(bytes[0]))
	return nil
}

func packUint8(value interface{}, writer io.Writer) error {
	var err error
	v := byte(value.(uint8))
	_, err = writer.Write([]byte{v})
	return err
}

func unpackUint8(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 1)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetUint(uint64(bytes[0]))
	return nil
}

func packInt16(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, uint16(value.(int16)))
	_, err = writer.Write(bytes)
	return err
}

func unpackInt16(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 2)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetInt(int64(binary.LittleEndian.Uint16(bytes)))
	return nil
}

func packUint16(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 2)
	binary.LittleEndian.PutUint16(bytes, value.(uint16))
	_, err = writer.Write(bytes)
	return err
}

func unpackUint16(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 2)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetUint(uint64(binary.LittleEndian.Uint16(bytes)))
	return nil
}

func packInt32(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(value.(int32)))
	_, err = writer.Write(bytes)
	return err
}

func unpackInt32(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 4)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetInt(int64(binary.LittleEndian.Uint32(bytes)))
	return nil
}

func packUint32(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, value.(uint32))
	_, err = writer.Write(bytes)
	return err
}

func unpackUint32(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 4)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetUint(uint64(binary.LittleEndian.Uint32(bytes)))
	return nil
}

func packInt64(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(value.(int64)))
	_, err = writer.Write(bytes)
	return err
}

func unpackInt64(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 8)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetInt(int64(binary.LittleEndian.Uint64(bytes)))
	return nil
}

func packUint64(value interface{}, writer io.Writer) error {
	var err error
	var bytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, value.(uint64))
	_, err = writer.Write(bytes)
	return err
}

func unpackUint64(val interface{}, reader io.Reader) error {
	var bytes = make([]byte, 8)
	_, err := reader.Read(bytes)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.SetUint(binary.LittleEndian.Uint64(bytes))
	return nil
}

func packVaruint32(value interface{}, writer io.Writer) error {
	var err error
	v := value.(uint32)
	for {
		var b = uint8(v) & 0x7f
		v >>= 7
		if v > 0 {
			b |= 0x80
		}

		_, err = writer.Write([]byte{b})
		if err != nil {
			break
		}

		if v == 0 {
			break
		}
	}
	return err
}

func unpackVaruint32(val interface{}, reader io.Reader) error {
	var vn uint64 = 0
	var by uint8 = 0
	for {
		bytes := make([]byte, 1)
		_, err := reader.Read(bytes)
		if err != nil {
			return err
		}

		vn |= uint64(bytes[0]&0x7f) << by
		by += 7

		if (bytes[0] & 0x80) == 0 {
			break
		}
	}

	v := reflect.ValueOf(val).Elem()
	v.SetUint(vn)
	return nil
}

func packAddress(value interface{}, writer io.Writer) error {
	var err error
	v := value.(common.Address)
	_, err = writer.Write(v[:])
	return err
}

func unpackAddress(val interface{}, reader io.Reader) error {
	var address common.Address
	_, err := reader.Read(address[:])
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.Set(reflect.ValueOf(address))
	return nil
}

func packString(value interface{}, writer io.Writer) error {
	var err error
	v := value.(string)
	err = packVaruint32(uint32(len(v)), writer)
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte(v))
	return err
}

func unpackString(val interface{}, reader io.Reader) error {
	var len uint32
	err := unpackVaruint32(&len, reader)
	if err != nil {
		return err
	}

	bytes := make([]byte, len)
	reader.Read(bytes)
	v := reflect.ValueOf(val).Elem()
	v.Set(reflect.ValueOf(string(bytes)))
	return nil
}

func packChecksum256(value interface{}, writer io.Writer) error {
	var err error
	v := value.(common.Hash)
	_, err = writer.Write(v[:])
	return err
}

func unpackChecksum256(val interface{}, reader io.Reader) error {
	var hash common.Hash
	_, err := reader.Read(hash[:])
	if err != nil {
		return err
	}

	v := reflect.ValueOf(val).Elem()
	v.Set(reflect.ValueOf(hash))
	return nil
}
