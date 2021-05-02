package main

type PlatformVariant int

const (
	PlatformUnknown PlatformVariant = iota
	PlatformAArch64
	PlatformX86_64
)
