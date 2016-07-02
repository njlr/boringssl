// Copyright (c) 2016, Google Inc.
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
// SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION
// OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN
// CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE. */

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
)

type test struct {
	LineNumber int
	Type       string
	Values     map[string]*big.Int
}

type testScanner struct {
	scanner *bufio.Scanner
	lineNo  int
	err     error
	test    test
}

func newTestScanner(r io.Reader) *testScanner {
	return &testScanner{scanner: bufio.NewScanner(r)}
}

func (s *testScanner) scanLine() bool {
	if !s.scanner.Scan() {
		return false
	}
	s.lineNo++
	return true
}

func (s *testScanner) addAttribute(line string) (key string, ok bool) {
	fields := strings.SplitN(line, "=", 2)
	if len(fields) != 2 {
		s.setError(errors.New("invalid syntax"))
		return "", false
	}

	key = strings.TrimSpace(fields[0])
	value := strings.TrimSpace(fields[1])

	valueInt, ok := new(big.Int).SetString(value, 16)
	if !ok {
		s.setError(fmt.Errorf("could not parse %q", value))
		return "", false
	}
	if _, dup := s.test.Values[key]; dup {
		s.setError(fmt.Errorf("duplicate key %q", key))
		return "", false
	}
	s.test.Values[key] = valueInt
	return key, true
}

func (s *testScanner) Scan() bool {
	s.test = test{
		Values: make(map[string]*big.Int),
	}

	// Scan until the first attribute.
	for {
		if !s.scanLine() {
			return false
		}
		if len(s.scanner.Text()) != 0 && s.scanner.Text()[0] != '#' {
			break
		}
	}

	var ok bool
	s.test.Type, ok = s.addAttribute(s.scanner.Text())
	if !ok {
		return false
	}
	s.test.LineNumber = s.lineNo

	for s.scanLine() {
		if len(s.scanner.Text()) == 0 {
			break
		}

		if s.scanner.Text()[0] == '#' {
			continue
		}

		if _, ok := s.addAttribute(s.scanner.Text()); !ok {
			return false
		}
	}
	return s.scanner.Err() == nil
}

func (s *testScanner) Test() test {
	return s.test
}

func (s *testScanner) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.scanner.Err()
}

func (s *testScanner) setError(err error) {
	s.err = fmt.Errorf("line %d: %s", s.lineNo, err)
}

func checkKeys(t test, keys ...string) bool {
	var foundErrors bool

	for _, k := range keys {
		if _, ok := t.Values[k]; !ok {
			fmt.Fprintf(os.Stderr, "Line %d: missing key %q.\n", t.LineNumber, k)
			foundErrors = true
		}
	}

	for k, _ := range t.Values {
		var found bool
		for _, k2 := range keys {
			if k == k2 {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Line %d: unexpected key %q.\n", t.LineNumber, k)
			foundErrors = true
		}
	}

	return !foundErrors
}

func checkResult(t test, expr, key string, r *big.Int) {
	if t.Values[key].Cmp(r) != 0 {
		fmt.Fprintf(os.Stderr, "Line %d: %s did not match %s.\n\tGot %s\n", t.LineNumber, expr, key, r.Text(16))
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s bn_tests.txt\n", os.Args[0])
		os.Exit(1)
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %s.\n", os.Args[0], err)
		os.Exit(1)
	}
	defer in.Close()

	scanner := newTestScanner(in)
	for scanner.Scan() {
		test := scanner.Test()
		switch test.Type {
		case "Sum":
			if checkKeys(test, "A", "B", "Sum") {
				r := new(big.Int).Add(test.Values["A"], test.Values["B"])
				checkResult(test, "A + B", "Sum", r)
			}
		case "LShift1":
			if checkKeys(test, "A", "LShift1") {
				r := new(big.Int).Add(test.Values["A"], test.Values["A"])
				checkResult(test, "A + A", "LShift1", r)
			}
		case "LShift":
			if checkKeys(test, "A", "N", "LShift") {
				r := new(big.Int).Lsh(test.Values["A"], uint(test.Values["N"].Uint64()))
				checkResult(test, "A << N", "LShift", r)
			}
		case "RShift":
			if checkKeys(test, "A", "N", "RShift") {
				r := new(big.Int).Rsh(test.Values["A"], uint(test.Values["N"].Uint64()))
				checkResult(test, "A >> N", "RShift", r)
			}
		case "Square":
			if checkKeys(test, "A", "Square") {
				r := new(big.Int).Mul(test.Values["A"], test.Values["A"])
				checkResult(test, "A * A", "Square", r)
			}
		case "Product":
			if checkKeys(test, "A", "B", "Product") {
				r := new(big.Int).Mul(test.Values["A"], test.Values["B"])
				checkResult(test, "A * B", "Product", r)
			}
		case "Quotient":
			if checkKeys(test, "A", "B", "Quotient", "Remainder") {
				q, r := new(big.Int).QuoRem(test.Values["A"], test.Values["B"], new(big.Int))
				checkResult(test, "A / B", "Quotient", q)
				checkResult(test, "A % B", "Remainder", r)
			}
		default:
			fmt.Fprintf(os.Stderr, "Line %d: unknown test type %q.\n", test.LineNumber, test.Type)
		}
	}
	if scanner.Err() != nil {
		fmt.Fprintf(os.Stderr, "Error reading tests: %s.\n", scanner.Err())
	}
}
