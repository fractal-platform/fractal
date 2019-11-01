package abi

import (
	"fmt"
	"io"
	"reflect"

	"github.com/fractal-platform/fractal/common"
)

func Encode(val interface{}, writer io.Writer) error {
	typ := reflect.TypeOf(val)
	switch {
	case typ.Name() == "Address":
		return packAddress(val, writer)
	case typ.Name() == "Hash":
		return packChecksum256(val, writer)
	case typ.Kind() == reflect.Struct:
		return encodeStruct(val, writer)
	case typ.Kind() == reflect.Slice:
		return encodeSlice(val, writer)
	case typ.Kind() == reflect.Array:
		return encodeSlice(val, writer)
	case typ.Kind() == reflect.Bool:
		return packBool(val, writer)
	case typ.Kind() == reflect.Int8:
		return packInt8(val, writer)
	case typ.Kind() == reflect.Uint8:
		return packUint8(val, writer)
	case typ.Kind() == reflect.Int16:
		return packInt16(val, writer)
	case typ.Kind() == reflect.Uint16:
		return packUint16(val, writer)
	case typ.Kind() == reflect.Int32:
		return packInt32(val, writer)
	case typ.Kind() == reflect.Uint32:
		return packUint32(val, writer)
	case typ.Kind() == reflect.Int64:
		return packInt64(val, writer)
	case typ.Kind() == reflect.Uint64:
		return packUint64(val, writer)
	case typ.Kind() == reflect.String:
		return packString(val, writer)
	}
	return fmt.Errorf("unsupported encode type: %s", typ.Name())
}

func encodeStruct(val interface{}, writer io.Writer) error {
	typ := reflect.TypeOf(val)
	v := reflect.ValueOf(val)
	for i := 0; i < typ.NumField(); i++ {
		err := Encode(v.Field(i).Interface(), writer)
		if err != nil {
			return err
		}
	}
	return nil
}

func encodeSlice(val interface{}, writer io.Writer) error {
	v := reflect.ValueOf(val)
	err := packVaruint32(uint32(v.Len()), writer)
	if err != nil {
		return err
	}

	for i := 0; i < v.Len(); i++ {
		err := Encode(v.Index(i).Interface(), writer)
		if err != nil {
			return err
		}
	}
	return nil
}

func Decode(val interface{}, reader io.Reader) error {
	var typ reflect.Type
	var v reflect.Value
	var isPointer bool
	if reflect.TypeOf(val).Kind() == reflect.Ptr {
		isPointer = true
		typ = reflect.ValueOf(val).Elem().Type()
	} else {
		isPointer = false
		v = val.(reflect.Value)
		typ = v.Type()
	}

	switch {
	case typ.Name() == "Address":
		if isPointer {
			return unpackAddress(val, reader)
		} else {
			var r common.Address
			err := unpackAddress(&r, reader)
			v.Set(reflect.ValueOf(r))
			return err
		}
	case typ.Name() == "Hash":
		if isPointer {
			return unpackChecksum256(val, reader)
		} else {
			var r common.Hash
			err := unpackChecksum256(&r, reader)
			v.Set(reflect.ValueOf(r))
			return err
		}
	case typ.Kind() == reflect.Struct:
		return decodeStruct(val, reader)
	case typ.Kind() == reflect.Slice:
		return decodeSlice(val, reader)
	case typ.Kind() == reflect.Array:
		return decodeArray(val, reader)
	case typ.Kind() == reflect.Bool:
		if isPointer {
			return unpackBool(val, reader)
		} else {
			var r bool
			err := unpackBool(&r, reader)
			v.SetBool(r)
			return err
		}
	case typ.Kind() == reflect.Int8:
		if isPointer {
			return unpackInt8(val, reader)
		} else {
			var r int8
			err := unpackInt8(&r, reader)
			v.SetInt(int64(r))
			return err
		}
	case typ.Kind() == reflect.Uint8:
		if isPointer {
			return unpackUint8(val, reader)
		} else {
			var r uint8
			err := unpackUint8(&r, reader)
			v.SetUint(uint64(r))
			return err
		}
	case typ.Kind() == reflect.Int16:
		if isPointer {
			return unpackInt16(val, reader)
		} else {
			var r int16
			err := unpackInt16(&r, reader)
			v.SetInt(int64(r))
			return err
		}
	case typ.Kind() == reflect.Uint16:
		if isPointer {
			return unpackUint16(val, reader)
		} else {
			var r uint16
			err := unpackUint16(&r, reader)
			v.SetUint(uint64(r))
			return err
		}
	case typ.Kind() == reflect.Int32:
		if isPointer {
			return unpackInt32(val, reader)
		} else {
			var r int32
			err := unpackInt32(&r, reader)
			v.SetInt(int64(r))
			return err
		}
	case typ.Kind() == reflect.Uint32:
		if isPointer {
			return unpackUint32(val, reader)
		} else {
			var r uint32
			err := unpackUint32(&r, reader)
			v.SetUint(uint64(r))
			return err
		}
	case typ.Kind() == reflect.Int64:
		if isPointer {
			return unpackInt64(val, reader)
		} else {
			var r int64
			err := unpackInt64(&r, reader)
			v.SetInt(int64(r))
			return err
		}
	case typ.Kind() == reflect.Uint64:
		if isPointer {
			return unpackUint64(val, reader)
		} else {
			var r uint64
			err := unpackUint64(&r, reader)
			v.SetUint(uint64(r))
			return err
		}
	case typ.Kind() == reflect.String:
		if isPointer {
			return unpackString(val, reader)
		} else {
			var r string
			err := unpackString(&r, reader)
			v.SetString(r)
			return err
		}
	}
	return fmt.Errorf("unsupported decode type: %s", typ.Name())
}

func decodeStruct(val interface{}, reader io.Reader) error {
	var typ reflect.Type
	var v reflect.Value
	if reflect.TypeOf(val).Kind() == reflect.Ptr {
		v = reflect.ValueOf(val).Elem()
		typ = reflect.ValueOf(val).Elem().Type()
	} else {
		v = val.(reflect.Value)
		typ = v.Type()
	}

	for i := 0; i < typ.NumField(); i++ {
		fv := v.Field(i)
		err := Decode(fv, reader)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeSlice(val interface{}, reader io.Reader) error {
	var typ reflect.Type
	var v reflect.Value
	if reflect.TypeOf(val).Kind() == reflect.Ptr {
		v = reflect.ValueOf(val).Elem()
		typ = reflect.ValueOf(val).Elem().Type()
	} else {
		v = val.(reflect.Value)
		typ = v.Type()
	}

	var len uint32
	err := unpackVaruint32(&len, reader)
	if err != nil {
		return err
	}

	rv := reflect.MakeSlice(typ, int(len), int(len))
	for i := 0; i < int(len); i++ {
		err = Decode(rv.Index(i), reader)
		if err != nil {
			return err
		}
	}
	v.Set(rv)
	return nil
}

func decodeArray(val interface{}, reader io.Reader) error {
	var v reflect.Value
	if reflect.TypeOf(val).Kind() == reflect.Ptr {
		v = reflect.ValueOf(val).Elem()
	} else {
		v = val.(reflect.Value)
	}

	var len uint32
	err := unpackVaruint32(&len, reader)
	if err != nil {
		return err
	}

	for i := 0; i < int(len); i++ {
		err = Decode(v.Index(i), reader)
		if err != nil {
			return err
		}
	}
	return nil
}
