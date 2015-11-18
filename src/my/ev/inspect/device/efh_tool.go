// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package device

type efh_toolDevice struct{}

func NewEfh_toolDevice() (dev *efh_toolDevice, err error) {
	// TODO
	return
}

func (_ *efh_toolDevice) ReadRegister(bar int, addr uint64, size int) (value uint64, err error) {
	// TODO
	return
}
