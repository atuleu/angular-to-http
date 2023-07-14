package ath

import (
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *ConfigSuite) TestByteSize(c *C) {
	testdata := []struct {
		Size  ByteSize
		Value string
		Error string
	}{
		{1, "1  ", ""},
		{2048, " 2k", ""},
		{3_145_728, " 3M ", ""},
		{4_294_967_296, " 4G", ""},
		{5_497_558_138_880, "5T", ""},
		{0, "1x", "invalid suffix 'x' in '1x'"},
		{0, "1 k", "invalid format for '1 k'"},
	}

	for _, d := range testdata {
		comment := Commentf("Testing %+v", d)

		var res ByteSize
		err := res.UnmarshalFlag(d.Value)

		if len(d.Error) == 0 {
			c.Check(err, IsNil, comment)
			c.Check(res, Equals, d.Size, comment)

			out, err := d.Size.MarshalFlag()
			c.Check(out, Equals, strings.TrimSpace(d.Value), comment)
			c.Check(err, IsNil, comment)
		} else {
			c.Check(res, Equals, ByteSize(0), comment)
			c.Check(err, ErrorMatches, d.Error)
		}
	}

}
