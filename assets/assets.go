// Package assets contains resources embedded in the application.
package assets

import _ "embed"

//go:embed icon.svg
var IconSVG []byte
