package abi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/utils/log"
)

type AbiSerializer struct {
	abiDef  AbiDef
	types   map[string]AbiType
	structs map[string]AbiStruct
	tables  map[string]AbiTable
}

func NewAbiSerializer(abi string) (*AbiSerializer, error) {
	var s AbiSerializer
	err := json.Unmarshal([]byte(abi), &s.abiDef)
	if err != nil {
		return nil, err
	}

	s.types = make(map[string]AbiType)
	for _, typ := range s.abiDef.Types {
		s.types[typ.NewTypeName] = typ
	}

	s.structs = make(map[string]AbiStruct)
	for _, st := range s.abiDef.Structs {
		s.structs[st.Name] = st
	}

	s.tables = make(map[string]AbiTable)
	for _, tbl := range s.abiDef.Tables {
		s.tables[tbl.Name] = tbl
	}

	return &s, nil
}

func (s *AbiSerializer) Serialize(data interface{}, typeName string, w io.Writer) error {
	var err error
	switch {
	case typeName == "bool":
		err = packBool(data, w)
	case typeName == "int8":
		d := data.(float64)
		err = packInt8(int8(d), w)
	case typeName == "uint8":
		d := data.(float64)
		err = packUint8(uint8(d), w)
	case typeName == "int16":
		d := data.(float64)
		err = packInt16(int16(d), w)
	case typeName == "uint16":
		d := data.(float64)
		err = packUint16(uint16(d), w)
	case typeName == "int32":
		d := data.(float64)
		err = packInt32(int32(d), w)
	case typeName == "uint32":
		d := data.(float64)
		err = packUint32(uint32(d), w)
	case typeName == "int64":
		d := data.(float64)
		err = packInt64(int64(d), w)
	case typeName == "uint64":
		d := data.(float64)
		err = packUint64(uint64(d), w)
	case typeName == "varuint32":
		err = packVaruint32(data, w)
	case typeName == "address":
		if reflect.TypeOf(data).Kind() == reflect.Slice {
			data = data.([]interface{})[0]
		}
		if reflect.TypeOf(data).Kind() == reflect.String {
			addressByte, err := hexutil.Decode(data.(string))
			if err != nil {
				log.Error("can not decode the address string", "err", err)
				return err
			}
			if len(addressByte) != common.AddressLength {
				log.Error("The length of address not equal 20.")
				return errors.New("The length of checksum256 not equal to 32.")
			}
			var addr common.Address
			copy(addr[:], addressByte)
			data = addr
		}

		err = packAddress(data, w)
	case typeName == "checksum256":
		if reflect.TypeOf(data).Kind() == reflect.Slice {
			data = data.([]interface{})[0]
		}
		if reflect.TypeOf(data).Kind() == reflect.String {
			hashByte, err := hexutil.Decode(data.(string))
			if err != nil {
				log.Error("can not decode the checksum string", "err", err)
				return err
			}
			if len(hashByte) != common.HashLength {
				log.Error("The length of checksum256 not equal 32.")
				return errors.New("The length of checksum256 not equal to 32.")
			}
			var hash common.Hash
			copy(hash[:], hashByte)
			data = hash
		}
		err = packChecksum256(data, w)
	case typeName == "string":
		err = packString(data, w)
	default:
		t, ok := s.types[typeName]
		if ok {
			err = s.Serialize(data, t.Type, w)
			break
		}

		st, ok := s.structs[typeName]
		if ok {
			for i, field := range st.Fields {
				m := data.([]interface{})
				err = s.Serialize(m[i], field.Type, w)
				if err != nil {
					break
				}
			}
			break
		}

		return fmt.Errorf("unknown type %s", typeName)
	}
	return err
}

func (s *AbiSerializer) GetTableKeyType(table string) string {
	return s.tables[table].KeyType
}
