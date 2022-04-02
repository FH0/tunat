package device

import (
	_ "embed"
)

//go:embed wintun/bin/arm/wintun.dll
var wintunDLL []byte
