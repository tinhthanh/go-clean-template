// Package api holds the contract the client is built to (openapi.json, the
// client repository's own api/openapi.json, copied by make contract-sync).
// The REST layer routes every operation in it (internal/controller/restapi/clientapi).
package api

import _ "embed"

//go:embed openapi.json
var spec []byte

// Spec is the contract, as the client wrote it.
func Spec() []byte { return spec }
