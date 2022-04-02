package device

import (
	_ "embed"
)

//go:embed wintun/bin/amd64/wintun.dll
var wintunDLL []byte
