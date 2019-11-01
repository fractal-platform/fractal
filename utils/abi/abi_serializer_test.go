package abi

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/fractal-platform/fractal/common"
	. "github.com/smartystreets/goconvey/convey"
)

const tokenAbi = `{
    "____comment": "This file was generated with fractal-abigen. DO NOT EDIT ",
    "version": "ftl::abi/0.3.0",
    "types": [],
    "structs": [
        {
            "name": "account",
            "base": "",
            "fields": [
                {
                    "name": "balance",
                    "type": "uint64"
                }
            ]
        },
        {
            "name": "account_key",
            "base": "",
            "fields": [
                {
                    "name": "owner",
                    "type": "address"
                },
                {
                    "name": "symbol",
                    "type": "string"
                }
            ]
        },
        {
            "name": "create",
            "base": "",
            "fields": [
                {
                    "name": "symbol",
                    "type": "string"
                },
                {
                    "name": "max_supply",
                    "type": "uint64"
                }
            ]
        },
        {
            "name": "issue",
            "base": "",
            "fields": [
                {
                    "name": "to",
                    "type": "address"
                },
                {
                    "name": "symbol",
                    "type": "string"
                },
                {
                    "name": "amount",
                    "type": "uint64"
                }
            ]
        },
        {
            "name": "retire",
            "base": "",
            "fields": [
                {
                    "name": "from",
                    "type": "address"
                },
                {
                    "name": "symbol",
                    "type": "string"
                },
                {
                    "name": "amount",
                    "type": "uint64"
                }
            ]
        },
        {
            "name": "stat",
            "base": "",
            "fields": [
                {
                    "name": "supply",
                    "type": "uint64"
                },
                {
                    "name": "max_supply",
                    "type": "uint64"
                },
                {
                    "name": "issuer",
                    "type": "address"
                }
            ]
        },
        {
            "name": "transfer",
            "base": "",
            "fields": [
                {
                    "name": "to",
                    "type": "address"
                },
                {
                    "name": "symbol",
                    "type": "string"
                },
                {
                    "name": "amount",
                    "type": "uint64"
                }
            ]
        }
    ],
    "actions": [
        {
            "name": "create",
            "type": "create"
        },
        {
            "name": "issue",
            "type": "issue"
        },
        {
            "name": "retire",
            "type": "retire"
        },
        {
            "name": "transfer",
            "type": "transfer"
        }
    ],
    "tables": [
        {
            "name": "ac",
            "key_type": "account_key",
            "value_type": "account"
        },
        {
            "name": "st",
            "key_type": "string",
            "value_type": "stat"
        }
    ],
    "variants": []
}`

func TestAbiSerializer(t *testing.T) {
	Convey("abi serializer", t, func() {
		s, err := NewAbiSerializer(tokenAbi)
		So(err, ShouldBeNil)

		var data interface{}
		err = json.Unmarshal([]byte("[\"ABC\", 100]"), &data)
		So(err, ShouldBeNil)

		bs := make([]byte, 0)
		w := bytes.NewBuffer(bs)
		err = s.Serialize(data, "create", w)
		So(err, ShouldBeNil)
		So(w.Bytes(), ShouldResemble, common.Hex2Bytes("034142436400000000000000"))
	})
}
