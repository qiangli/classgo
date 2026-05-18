// namelist-convert reprojects a header-bearing student namelist XLSX into the
// 16-column positional layout consumed by the admin import flow (see
// internal/datastore/namelist.go: ParseNamelist).
//
// Usage:
//
//	namelist-convert -in <src.xlsx> -out <dst.xlsx>
package main

import (
	"flag"
	"fmt"
	"os"

	"classgo/internal/datastore"
)

func main() {
	in := flag.String("in", "", "source XLSX with header row (Student Name, English Name, ...)")
	out := flag.String("out", "", "destination XLSX (16-column positional layout)")
	flag.Parse()

	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: namelist-convert -in <src.xlsx> -out <dst.xlsx>")
		os.Exit(2)
	}

	n, err := datastore.ConvertNamelistXLSX(*in, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("converted %d rows -> %s\n", n, *out)
}
