// Package platformident provides identification of the platform this application was built for.
// It does _not_ currently do runtime checks, because there's no need to.
package platformident

type PlatformVariant int

const (
	PlatformUnknown PlatformVariant = iota
	PlatformAArch64
	PlatformX86_64
)
