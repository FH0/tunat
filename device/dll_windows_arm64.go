package device

import (
	_ "embed"
)

//go:embed wintun/bin/arm64/wintun.dll
var wintunDLL []byte
