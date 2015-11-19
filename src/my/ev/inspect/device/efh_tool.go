// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package device

import (
	"fmt"
	"os/exec"

	"github.com/ikravets/errs"
)

type efh_toolDevice struct{}

func NewEfh_toolDevice() (dev *efh_toolDevice, err error) {
	return &efh_toolDevice{}, nil
}

func (_ *efh_toolDevice) ReadRegister(bar int, addr uint64, size int) (value uint64, err error) {
	errs.PassE(&err)
	v, err := (exec.Command("efh_tool", "read", fmt.Sprint(bar), fmt.Sprintf("%0#16x", addr), fmt.Sprint(size))).Output()
	errs.CheckE(err)
	_, err = fmt.Sscan(string(v), &value)
	return
}
