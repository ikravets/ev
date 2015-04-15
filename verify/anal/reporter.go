// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package anal

import (
	"fmt"
	"os"
	"path/filepath"

	"my/errs"
)

type Reporter struct {
	outDir   string
	analyzer *Analyzer
}

func NewReporter() *Reporter {
	return &Reporter{}
}

func (r *Reporter) SetOutputDir(path string) {
	r.outDir = path
	errs.CheckE(os.MkdirAll(r.outDir, 0755))
}
func (r *Reporter) SetAnalyzer(a *Analyzer) {
	r.analyzer = a
}
func (r *Reporter) SaveAll() {
	r.SaveBookSizeHistogram()
	r.SaveOrderCollisionsHistogram()
}
func (r *Reporter) SaveBookSizeHistogram() {
	errs.Check(r.analyzer != nil)
	fileName := filepath.Join(r.outDir, "book_size_hist.tsv")
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	errs.CheckE(err)
	defer file.Close()
	bsh := r.analyzer.BookSizeHist()
	_, err = fmt.Fprintf(file, "size\tbooks\tsample\n")
	errs.CheckE(err)
	for _, bsv := range bsh {
		_, err = fmt.Fprintf(file, "%d\t%d\t%v\n", bsv.Levels, bsv.OptionNumber, bsv.Sample)
		errs.CheckE(err)
	}
}
func (r *Reporter) SaveOrderCollisionsHistogram() {
	errs.Check(r.analyzer != nil)
	orderHists := r.analyzer.OrdersHashCollisionHist()
	for i, ohs := range orderHists {
		fileName := filepath.Join(r.outDir, fmt.Sprintf("order_collision_hist_%d.tsv", i))
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		errs.CheckE(err)
		defer file.Close()
		_, err = fmt.Fprintf(file, "maxCollisions\tbuckets\n")
		errs.CheckE(err)
		for _, h := range ohs {
			fmt.Fprintf(file, "%d\t%d\n", h.Bin, h.Count)
		}
	}
}
