package main

import (
	"fmt"
	"testing"
)

var condData = map[string]string{
	"title":  "SOME is titile",
	"descr":  "OTHER is description",
	"first":  "first description",
	"second": "second is description",
	"third":  "third is description",
}

var condTests = []struct {
	in  string
	out bool
}{
	{
		in:  "'SOME' in {{title}}",
		out: true,
	},
	{
		in:  "'SOME' in prefix {{title}}",
		out: true,
	},
	{
		in:  "'2SOME' in prefix {{title}}",
		out: false,
	},
	{
		in:  "'SOME' in suffix {{title}}",
		out: false,
	},

	{
		in:  "'SOME' not in {{title}}",
		out: false,
	},
	{
		in:  "'SOME' in {{title}} and 'OTHER' in {{descr}} ",
		out: true,
	},
	{
		in:  "'SOME' in {{title}} and 'OTHER' not in {{descr}} ",
		out: false,
	},
	{
		in:  "'SOME' in {{title}} or 'OTHER' not in {{descr}} ",
		out: true,
	},
	{
		in:  "('first' in {{first}} or 'second' not in {{second}}) and 'third' in {{third}}",
		out: true,
	},
	{
		in:  "'first' in {{first}} and ('second' not in {{second}} and 'third' in {{third}})",
		out: false,
	},
}

var condFailTest = []struct {
	in string
}{
	{
		in: "(('SOME' not in {{title}})",
	},
	{
		in: "('SOME' not in {{title}})))",
	},
	{
		in: "'SOME' in not {{title}}",
	},
	{
		in: "in 'SOME' in {{title}}",
	},
	{
		in: "not 'SOME' in {{title}}",
	},
}

var formatData = map[string]string{
	"one":    "ONE",
	"second": "SECOND",
}

var formatTest = []struct {
	in  string
	out string
}{
	{
		in:  "{{one}}/{{second}}/{{one}}",
		out: "ONE/SECOND/ONE",
	},
	{
		in:  "{{one}}/real/{{second}}/real/{{one}}",
		out: "ONE/real/SECOND/real/ONE",
	},
	{
		in:  "{{one}}/Media Path/{{second}}/real/{{one}}",
		out: "ONE/Media Path/SECOND/real/ONE",
	},
	{
		in:  "$HOME/{{one}}/Media Path/{{second}}/real/{{one}}",
		out: "$HOME/ONE/Media Path/SECOND/real/ONE",
	},
	{
		in:  "",
		out: "",
	},
}

func TestCond(t *testing.T) {
	fmt.Println("TestCond")
	for i, test := range condTests {
		fmt.Println("t:", i, "tpl :", test.in)
		r, err := EvalFilter(test.in, condData)
		if err != nil {
			t.Error("Parse template failed :", err)
			continue
		}
		if r != test.out {

			t.Error("t:", i, "expected :", test.out, "got :", r)
		}
	}
}

func TestFailCond(t *testing.T) {
	fmt.Println("TestFailCond")
	for i, test := range condFailTest {
		fmt.Println("t:", i, "tpl :", test.in)
		_, err := EvalFilter(test.in, condData)
		if err != nil {
			continue // error is good
		} else {
			t.Error("t:", i, "expected : ERROR")
		}

	}
}

func TestFormat(t *testing.T) {
	fmt.Println("TestFormat")
	for i, test := range formatTest {
		fmt.Println("t:", i, "tpl :", test.in)
		r := EvalFormat(test.in, formatData)
		if r != test.out {
			t.Error("t:", i, "expected :", test.out, "got :", r)
		}
	}
}
