package main

import (
	"testing"
)

func Test_main(t *testing.T) {

	t.Skip()

	c := Config{
		Test:    "remove",
		Base:    "",
		N:       1_000_000,
		Workers: 4,
	}

	//TestRemove(c)
	TestInsert(c)

}
