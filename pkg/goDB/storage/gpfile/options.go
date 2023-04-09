package gpfile

import (
	"io/fs"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
)

// Option defines optional arguments to gpfile
type Option func(any)

// optionSetterCommon denotes options that apply to both GPDir and GPFile
type optionSetterCommon interface {
	setPermissions(fs.FileMode)
}

// optionSetterFile denotes options that apply to GPFile only
type optionSetterFile interface {
	optionSetterCommon
	setMemPool(MemPoolGCable)
	setEncoder(encoder.Encoder)
}

// WithEncoder allows to set the compression implementation
func WithEncoder(e encoder.Encoder) Option {
	return func(o any) {
		if obj, ok := o.(optionSetterFile); ok {
			obj.setEncoder(e)
		}
	}
}

// WithReadAll triggers a full read of the underlying file from disk
// upon first read access to minimize I/O load.
// Seeking is handled by replacing the underlying file with a seekable
// in-memory structure (c.f. readWriteSeekCloser interface)
func WithReadAll(pool MemPoolGCable) Option {
	return func(o any) {
		if obj, ok := o.(optionSetterFile); ok {
			obj.setMemPool(pool)
		}
	}
}

// WithPermissions sets a non-default set of permissions / file mode for
// the file
func WithPermissions(permissions fs.FileMode) Option {
	return func(o any) {
		if obj, ok := o.(optionSetterCommon); ok {
			obj.setPermissions(permissions)
		}
	}
}
