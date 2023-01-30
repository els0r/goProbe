//go:build darwin || linux
// +build darwin linux

package lz4cust

/*
#cgo linux LDFLAGS: ${SRCDIR}/liblz4_linux.a
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/liblz4_arm64_darwin.a
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/liblz4_amd64_darwin.a
*/
import "C"
