package abi

type AbiType struct {
	NewTypeName string `json:"new_type_name"    gencodec:"required"`
	Type        string `json:"type"             gencodec:"required"`
}

type AbiField struct {
	Name string `json:"name"    gencodec:"required"`
	Type string `json:"type"    gencodec:"required"`
}

type AbiStruct struct {
	Name   string     `json:"name"      gencodec:"required"`
	Base   string     `json:"base"      gencodec:"required"`
	Fields []AbiField `json:"fields"    gencodec:"required"`
}

type AbiAction struct {
	Name string `json:"name"      gencodec:"required"`
	Type string `json:"type"      gencodec:"required"`
}

type AbiTable struct {
	Name      string `json:"name"          gencodec:"required"`
	KeyType   string `json:"key_type"      gencodec:"required"`
	ValueType string `json:"value_type"    gencodec:"required"`
}

type AbiDef struct {
	Version string      `json:"version"     gencodec:"required"`
	Types   []AbiType   `json:"types"       gencodec:"required"`
	Structs []AbiStruct `json:"structs"     gencodec:"required"`
	Actions []AbiAction `json:"actions"     gencodec:"required"`
	Tables  []AbiTable  `json:"tables"      gencodec:"required"`
}
