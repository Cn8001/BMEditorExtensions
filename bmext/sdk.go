package bmext

import "embed"

// SDK is the part of this module an extension keeps a copy of in its own
// bmext/ folder, so that it builds on its own, from its own repository:
// frontend/ (the script's API types and Vite settings) as bmext/, and the
// backend SDK host/ as bmext/host/. bmx init writes the copy.
//
//go:embed frontend/bmeditor.d.ts frontend/vite.mjs host/host.go host/client.go
var SDK embed.FS
