package device

import (
	_ "embed"
)

//go:embed wintun/bin/x86/wintun.dll
var wintunDLL []byte
