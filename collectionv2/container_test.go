package collectionv2

import (
	"testing"

	"github.com/fulldump/biff"
)

func TestSliceContainer(t *testing.T) {

	biff.Alternative("SliceContainer", func(a *biff.A) {

		c := NewSliceContainer()

		a.Alternative("Insert", func(a *biff.A) {
			row1 := &Row{I: -1}
			c.ReplaceOrInsert(row1)
			biff.AssertEqual(row1.I, 0)
			biff.AssertEqual(c.Len(), 1)

			row2 := &Row{I: -1}
			c.ReplaceOrInsert(row2)
			biff.AssertEqual(row2.I, 1)
			biff.AssertEqual(c.Len(), 2)

			a.Alternative("Get", func(a *biff.A) {
				r, ok := c.Get(row1)
				biff.AssertTrue(ok)
				biff.AssertEqual(r, row1)

				r, ok = c.Get(row2)
				biff.AssertTrue(ok)
				biff.AssertEqual(r, row2)
			})

			a.Alternative("Has", func(a *biff.A) {
				biff.AssertTrue(c.Has(row1))
				biff.AssertTrue(c.Has(row2))
				biff.AssertFalse(c.Has(&Row{I: 999}))
			})

			a.Alternative("Delete", func(a *biff.A) {
				// Delete row1 (index 0)
				// Should move row2 (index 1) to index 0
				c.Delete(row1)

				biff.AssertEqual(c.Len(), 1)
				biff.AssertFalse(c.Has(row1))
				biff.AssertTrue(c.Has(row2))

				// Check that row2 index was updated
				biff.AssertEqual(row2.I, 0)

				// Check that slot 0 contains row2
				r, ok := c.Get(&Row{I: 0})
				biff.AssertTrue(ok)
				biff.AssertEqual(r, row2)
			})
		})

	})
}
