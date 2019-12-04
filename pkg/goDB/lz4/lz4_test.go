package lz4

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/pierrec/lz4"
)

var data = []byte("Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet. Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet.")

func BenchmarkCompress(b *testing.B) {
	encoder := New()

	tmpfile, err := ioutil.TempFile("", "compress")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	for n := 0; n < b.N; n++ {
		if _, err := encoder.Compress(data, tmpfile); err != nil {
			b.Fatalf("could not compress data: %v", err)
		}
	}
}

func BenchmarkCompress2(b *testing.B) {

	tmpfile, err := ioutil.TempFile("", "compress")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	encoder := lz4.NewWriter(tmpfile)

	for n:=0; n<b.N; n++ {
		if _, err := encoder.Write(data); err != nil {
			b.Fatalf("could not compress data: %v", err)
		}
	}
}
