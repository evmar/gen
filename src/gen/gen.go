package main

import (
	"flag"
	"fmt"
	"os"

	"gen/lex"
	"gen/lr"
)

var outpath = flag.String("o", "-", "output path")
var verbose = flag.Bool("v", false, "verbose output")

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func output(data []byte) error {
	f := os.Stdout
	var err error

	if *outpath != "-" {
		f, err = os.Create(*outpath)
		if err != nil {
			return err
		}
	}

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	if f != os.Stdout {
		err = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: gen [FLAGS] MODE INFILE

MODE is one of
  lex  generate a lexer
  lr   generate an lr parser

FLAGS are
`)
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	mode := flag.Arg(0)
	infile := flag.Arg(1)

	switch mode {
	case "lex":
		data, err := lex.Main(infile, *verbose)
		check(err)
		check(output(data))
	case "lr":
		data, err := lr.Main(infile, *verbose)
		check(err)
		check(output(data))
	default:
		check(fmt.Errorf("unknown mode %q", mode))
	}
}
