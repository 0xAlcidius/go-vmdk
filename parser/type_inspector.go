package parser

import (
	"encoding/json"
	"fmt"
)

const (
	MONOLITHICSPARSE            = "monolithicSparse"
	MONOLITHICFLAT              = "monolithicFlat"
	TWOGBMAXEXTENTSPARSE        = "twoGbMaxExtentSparse"
	TWOGBMAXEXTENTFLAT          = "twoGbMaxExtentFlat"
	STREAMOPTIMIZED             = "streamOptimized"
	VMFS                        = "vmfs"
	VMFSSPARSE                  = "vmfsSparse"
	VMFSRAW                     = "vmfsRaw"
	VMFSPASSTHROUGHRAWDEVICEMAP = "vmfsPassthroughRawDeviceMap"
	FULLDEVICE                  = "fullDevice"
	PARTITIONEDDEVICE           = "partitionedDevice"
	CUSTOM                      = "custom"
	Unknown                     = "unknown"
)

type VMDKConfig struct {
	VMDKVersion          string
	VMDKEncoding         string
	VMDKCid              string
	VMDKParentCid        string
	VMDKCreateType       string
	DBBAdatperType       string
	DBBGeometryCylinders string
	DBBGeometryHeads     string
	DBBGeometrySectors   string
	DBBLongContentId     string
	DBBUuid              string
	DBBVirtualHWVersion  string
}

func NewVMDKConfig() *VMDKConfig {
	return &VMDKConfig{
		VMDKVersion:          "1",
		VMDKEncoding:         "windows-1252",
		VMDKCid:              "0",
		VMDKParentCid:        "0",
		VMDKCreateType:       "unknown",
		DBBAdatperType:       "lsilogic",
		DBBGeometryCylinders: "0",
		DBBGeometryHeads:     "0",
		DBBGeometrySectors:   "0",
		DBBLongContentId:     "",
		DBBUuid:              "",
		DBBVirtualHWVersion:  "",
	}
}

func VMDKConfigSetters(config *VMDKConfig) map[string]func(string) {
	return map[string]func(string){
		"version":                func(val string) { config.VMDKVersion = val },
		"encoding":               func(val string) { config.VMDKEncoding = val },
		"CID":                    func(val string) { config.VMDKCid = val },
		"parentCID":              func(val string) { config.VMDKParentCid = val },
		"createType":             func(val string) { config.VMDKCreateType = val },
		"ddb.adapterType":        func(val string) { config.DBBAdatperType = val },
		"ddb.geometry.cylinders": func(val string) { config.DBBGeometryCylinders = val },
		"ddb.geometry.heads":     func(val string) { config.DBBGeometryHeads = val },
		"ddb.geometry.sectors":   func(val string) { config.DBBGeometrySectors = val },
		"ddb.longContentID":      func(val string) { config.DBBLongContentId = val },
		"ddb.uuid":               func(val string) { config.DBBUuid = val },
		"ddb.virtualHWVersion":   func(val string) { config.DBBVirtualHWVersion = val },
	}
}

func PrintVMDKConfig(config VMDKConfig) {
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Println("error printing VMDKConfig:", err)
		return
	}
	fmt.Println(string(out))
}
