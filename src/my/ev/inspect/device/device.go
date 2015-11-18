// Copyright (c) Ilia Kravets, 2015. All rights reserved. PROVIDED "AS IS"
// WITHOUT ANY WARRANTY, EXPRESS OR IMPLIED. See LICENSE file for details.

package device

type Device interface {
	ReadRegister(bar int, addr uint64, size int) (value uint64, err error)
}
