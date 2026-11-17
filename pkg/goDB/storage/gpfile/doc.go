// Package gpfile implements goDB's on-disk day directories: one GPDir per <iface>/<YYYY>/<MM>/<day epoch>_<meta suffix>, holding one .gpf column file per column and a binary .blockmeta index.
//
// GPFile appends compressed blocks; offsets and per-block encoder types live in the GPDir metadata.
package gpfile
