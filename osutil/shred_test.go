// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2015 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package osutil_test

import (
	"io/ioutil"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/osutil"
)

type ShredTestSuite struct {
	log []string
}

var _ = Suite(&ShredTestSuite{})

func (ts *ShredTestSuite) TestFileIsRemoved(c *C) {
	fname := filepath.Join(c.MkDir(), "randomfile")
	err := ioutil.WriteFile(fname, []byte(fname), 0644)
	c.Assert(err, IsNil)

	err = osutil.Shred(fname)
	c.Assert(err, IsNil)

	c.Assert(osutil.FileExists(fname), Equals, false)
}
