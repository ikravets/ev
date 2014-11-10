// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package main

import (
	"bufio"
	"fmt"
	"github.com/kr/pretty"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func mapSubstring(whole string, indexes []int) (text []string) {
	text = make([]string, len(indexes)/2)
	idx := make([]int, len(text)+1)
	for i := 0; i < len(idx)-1; i++ {
		idx[i] = indexes[i*2]
		if idx[i] >= len(whole) {
			idx[i] = len(whole)
		}
	}
	idx[len(idx)-1] = len(whole)
	for i := 0; i < len(text); i++ {
		text[i] = whole[idx[i]:idx[i+1]]
	}
	return
}

type TableRow struct {
	name   string
	offset int
	length int
	value  string
	notes  string
	codes  map[byte]string
}

func (r *TableRow) getKeys() string {
	if len(r.codes) == 0 {
		return ""
	}
	keys := make([]int, len(r.codes))
	i := 0
	for k := range r.codes {
		keys[i] = int(k)
		i++
	}
	sort.Ints(keys)
	bs := make([]byte, len(r.codes))
	for i, k := range keys {
		bs[i] = byte(k)
	}
	return string(bs)
}

type Table struct {
	caption string
	rows    []TableRow
}

func (t *Table) setCaption(text string) {
	spacesRegexp := regexp.MustCompile("\\s+")
	s := text
	s = spacesRegexp.ReplaceAllLiteralString(s, " ")
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.Title(s)
	t.caption = s
}

func (t *Table) getTypeChar() byte {
	ks := t.rows[0].getKeys()
	if len(ks) != 1 {
		log.Fatal("Unexpected first table row:", t.rows[0])
	}
	return ks[0]
}

func (t *Table) addRow(columnText []string) {
	spacesRegexp := regexp.MustCompile("\\s+")
	for i := 0; i < len(columnText)-1; i++ {
		columnText[i] = spacesRegexp.ReplaceAllLiteralString(columnText[i], " ")
	}
	var err error
	var r TableRow
	r.name = strings.TrimSpace(columnText[0])
	r.offset, err = strconv.Atoi(strings.TrimSpace(columnText[1]))
	if err != nil {
		log.Fatal(err)
	}
	r.length, err = strconv.Atoi(strings.TrimSpace(columnText[2]))
	if err != nil {
		log.Fatal(err)
	}
	r.value = strings.TrimSpace(columnText[3])
	if r.value == "Alphabetic" {
		r.value = "Alpha"
	}
	if r.value == "Alpha" {
		text := columnText[4]
		alphaCodesRegexp := regexp.MustCompile("(?m)^\\s*[“‘]?(\\w)[”’]? ?[=–-] ?")
		descriptions := alphaCodesRegexp.Split(text, -1)
		codes := alphaCodesRegexp.FindAllStringSubmatch(text, -1)
		if len(descriptions)-1 != len(codes) {
			log.Fatal("can't parse Alpha notes:", text)
		}
		m := make(map[byte]string)
		for i := range codes {
			c := codes[i][1][0]
			d := strings.TrimSpace(spacesRegexp.ReplaceAllLiteralString(descriptions[i+1], " "))
			m[c] = d
		}
		d := strings.TrimSpace(spacesRegexp.ReplaceAllLiteralString(descriptions[0], " "))
		allowableValuesRegexp := regexp.MustCompile("\\s*The allowable values are:\\s*")
		r.notes = allowableValuesRegexp.ReplaceAllLiteralString(d, "")
		r.codes = m
	} else {
		text := columnText[4]
		text = strings.TrimSpace(spacesRegexp.ReplaceAllLiteralString(text, " "))
		text = strings.Replace(text, " NOTE: When converted to a decimal format, this price is in fixed point format with 3 whole number places followed by 2 decimal digits.", "", 1)
		r.notes = text
	}
	t.rows = append(t.rows, r)
}

func superimposeStrings(s1, s2 string) string {
	rs := make([]rune, 0, len(s1)+len(s2))
	for _, c := range s1 {
		rs = append(rs, rune(c))
	}
	for i, c := range s2 {
		if i >= len(rs) {
			rs = append(rs, rune(c))
		} else if rs[i] == ' ' {
			rs[i] = rune(c)
		} else if c != ' ' {
			log.Fatalf("Cannot superimpose \"%s\" and \"%s\" at pos %d\n", s1, s2, i)
		}
	}
	return string(rs)
}

func adhocTextFix(scanner *bufio.Scanner) string {
	text := scanner.Text()

	r := regexp.MustCompile("(^\\s*Open State.*Alphabetic    ) (The allowable values are:)")
	text = r.ReplaceAllString(text, "$1$2")

	r2 := regexp.MustCompile("^(Number Delta {26})(associated with the new quote)$")
	text = r2.ReplaceAllString(text, "$1                   $2")

	r3 := regexp.MustCompile("^ (Ask|Bid) Reference {41}The (ask|bid) reference number delta$")
	if r3.MatchString(text) {
		scanner.Scan()
		text = superimposeStrings(text, scanner.Text())
	}

	r4 := regexp.MustCompile("^Total Number of")
	if r4.MatchString(text) {
		text = "Total Number of Reference 5 2 Integer                 The number of single side deletes in this"
		scanner.Scan()
		scanner.Scan()
	}

	r5 := regexp.MustCompile("^Reference {45}The order/quote side reference number")
	if r5.MatchString(text) {
		scanner.Scan()
		scanner.Scan()
		text = ""
	}

	return text
}

func parseEventCodes(scanner *bufio.Scanner, row *TableRow) {
	spacesRegexp := regexp.MustCompile("\\s+")
	tableRowRegexp := regexp.MustCompile("^ “(\\w)” {15}(\\w.*) {7}")
	tableContRegexp := regexp.MustCompile("^ {19}(\\w.*)")

	if len(row.codes) != 0 {
		log.Fatal("row.codes is non-empty:", row.codes)
	}

	row.codes = make(map[byte]string)
	scanner.Scan()
	for {
		match := tableRowRegexp.FindStringSubmatch(scanner.Text())
		if match == nil {
			break
		}
		key := match[1]
		descr := match[2]
		for scanner.Scan() {
			match := tableContRegexp.FindStringSubmatch(scanner.Text())
			if match == nil {
				break
			}
			descr += " " + match[1]
		}
		descr = strings.TrimSpace(spacesRegexp.ReplaceAllLiteralString(descr, " "))
		row.codes[key[0]] = descr
	}
}

func parseDoc(r io.Reader) (tables []Table) {
	scanner := bufio.NewScanner(r)

	eventCodesTableHeaderRegexp := regexp.MustCompile("Code *Explanation *When \\(typically\\)")

	tableHeaderRegexp := regexp.MustCompile("Name *Offset *Length *Value *Notes")
	tableFullRowRegexp := regexp.MustCompile("(.*)\\s+(\\d+)\\s+(\\d+)\\s+(\\w+)\\s+(.*)")
	emptyLineRegexp := regexp.MustCompile("^\\s*$")
	var lastText, caption string

	scanner.Scan()
docLoop:
	for {
		var table Table
		// find table start
		for {
			if tableHeaderRegexp.MatchString(scanner.Text()) {
				caption = lastText
				break
			}
			if eventCodesTableHeaderRegexp.MatchString(scanner.Text()) {
				lastTable := &tables[len(tables)-1]
				lastRow := &lastTable.rows[len(lastTable.rows)-1]
				parseEventCodes(scanner, lastRow)
			}
			lastText = scanner.Text()
			if !scanner.Scan() {
				break docLoop
			}
		}
		var columnIndexes []int
		var columnText []string
		var emptyLines = 0
		for scanner.Scan() {
			text := adhocTextFix(scanner)
			if emptyLineRegexp.MatchString(text) {
				if emptyLines < 2 {
					emptyLines++
					continue
				}
				// assume end of table
				table.addRow(columnText)
				emptyLines = 0
				break
			}
			emptyLines = 0
			ci := tableFullRowRegexp.FindStringSubmatchIndex(text)
			if ci != nil {
				if columnText != nil {
					// end of row
					table.addRow(columnText)
				}
				columnIndexes = ci
				columnText = mapSubstring(text, columnIndexes[2:])
			} else {
				ct := mapSubstring(text, columnIndexes[2:])
				if emptyLineRegexp.MatchString(ct[1]) && emptyLineRegexp.MatchString(ct[2]) {
					// table row continuation
					columnText[0] += "\n" + ct[0]
					columnText[3] += "\n" + ct[3]
					columnText[4] += "\n" + ct[4]
				} else if text == caption {
					// table wrap to the next page
					scanner.Scan()
					continue
				} else {
					// end of row, end of table
					table.addRow(columnText)
					break
				}
			}
		}
		table.setCaption(caption)
		tables = append(tables, table)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(scanner.Err())
	}
	return
}

func obtainInputFromPdf() (reader io.Reader, finisher func()) {
	cmdArgs := []string{
		"-layout",
		"-x", "70",
		"-y", "80",
		"-H", "640",
		"-W", "500",
		"-nopgbrk",
		"itto_spec_30.pdf",
		"-",
	}
	cmd := exec.Command("pdftotext", cmdArgs...)
	reader, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	finisher = func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}
	return
}

func obtainInputFromTxt() (reader io.Reader, finisher func()) {
	r, err := os.Open("itto_spec_30.txt")
	if err != nil {
		log.Fatal("Error opening input file:", err)
	}
	f := func() { r.Close() }
	return r, f
}

func showMessageTypes(tables []Table) {
	for _, t := range tables {
		char := string(t.rows[0].notes[3])
		fmt.Println(char, t.caption)
	}
}

func showWiresharkIttoImplH(tables []Table) {
	heading := `
/* this is a generated file */
#ifndef PACKET_ITTO_IMPL_H
#define PACKET_ITTO_IMPL_H

enum itto_item_type {
    ITTO_ITEM_TYPE_NONE,
    ITTO_ITEM_TYPE_ALPHA,
    ITTO_ITEM_TYPE_STRING,
    ITTO_ITEM_TYPE_INT,
};

struct itto_message_item {
    const char *name;
    int offset;
    unsigned length;
    enum itto_item_type type;
    const char *notes;
    int hf_id;
    const char *hf_abbrev;
    const value_string *hf_values;
};

#define ITTO_END_OF_ITEMS NULL, 0, 0, ITTO_ITEM_TYPE_NONE, NULL, 0, NULL, NULL

struct itto_message {
    const char *caption;
    struct itto_message_item *items;
};

static const value_string itto_values_BS[] = {
    {'B', "Buy"},
    {'S', "Sell"},
    {0, NULL},
};

static const value_string itto_values_NY[] = {
    {'N', "No"},
    {'Y', "Yes"},
    {0, NULL},
};

`
	footer := `
#endif /* PACKET_ITTO_IMPL_H */
`

	nonAlphaRegexp := regexp.MustCompile("\\W")
	totalItems := 0

	fmt.Print(heading)
	bsKeys := "BS"
	ynKeys := "NY"
	for _, t := range tables {
		for _, r := range t.rows[1:] {
			keys := r.getKeys()
			if len(keys) == 0 || keys == bsKeys || keys == ynKeys {
				continue
			}
			fmt.Printf("static const value_string itto_values_%s[] = {\n", keys)
			for _, k := range keys {
				fmt.Printf("    {'%c', \"%s\"},\n", k, r.codes[byte(k)])
			}
			fmt.Println("    {0, NULL},\n};\n")
		}
	}
	fmt.Println()
	for _, t := range tables {
		fmt.Printf("static struct itto_message_item itto_items_%c[] = {\n", t.getTypeChar())
		for _, r := range t.rows[1:] {
			var cType string
			var hfValues = "NULL"
			switch r.value {
			case "Alpha", "Alphabetic":
				cType = "ITTO_ITEM_TYPE_ALPHA"
				if len(r.codes) > 0 {
					hfValues = "itto_values_" + r.getKeys()
				}
			case "Integer", "Long Integer":
				cType = "ITTO_ITEM_TYPE_INT"
			case "Alphanumeric":
				cType = "ITTO_ITEM_TYPE_STRING"
			}
			if cType == "" {
				log.Println("WARNING: Unknown value type: ", r.value)
			}
			var hfAbbrev = "itto." + nonAlphaRegexp.ReplaceAllLiteralString(r.name, "")
			fmt.Printf("    {\"%s\", %d, %d, %s, \"%s\", -1, \"%s\", %s},\n",
				r.name, r.offset, r.length, cType, r.notes, hfAbbrev, hfValues)
			totalItems++
		}
		fmt.Printf("    {ITTO_END_OF_ITEMS}\n};\n\n")
	}
	fmt.Println("#define ITTO_TOTAL_ITEMS ", totalItems)
	fmt.Println("static const struct itto_message itto_messages[256] = {")
	for _, t := range tables {
		fmt.Printf("    ['%c'] = {\"%s\", itto_items_%c},\n", t.getTypeChar(), t.caption, t.getTypeChar())
	}
	fmt.Println("};")
	fmt.Print(footer)
}

func main() {
	reader, finisher := obtainInputFromPdf()
	defer finisher()
	tables := parseDoc(reader)
	/*
		for _, t := range tables {
				for _, r := range t.rows {
					fmt.Println(r.value)
						if r.value == "Alphanumeric" {
							fmt.Println(r.length)
						}
				}
			fmt.Println(t.caption)
		}
	*/
	//showMessageTypes(tables)
	showWiresharkIttoImplH(tables)
	//pretty.Println(tables)
	_ = pretty.Print
}
