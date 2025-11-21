package collectionv2

import (
	"sync/atomic"

	"github.com/fulldump/inceptiondb/utils"
)

type loadedCommand struct {
	seq            int
	cmd            *Command
	decodedPayload interface{}
	err            error
}

func LoadCollection(c *Collection) error {
	cmds, errs := c.storage.Load()

	for cmd := range cmds {
		switch cmd.Cmd.Name {
		case "insert":
			// Use decoded payload if available
			row := &Row{
				Payload: cmd.Cmd.Payload,
				Decoded: cmd.DecodedPayload,
			}
			err := c.addRow(row)
			if err != nil {
				return err
			}
			atomic.AddInt64(&c.Count, 1)
		case "remove":
			params := cmd.DecodedPayload.(struct{ I int })
			// Find row by I
			dummy := &Row{I: params.I}
			if c.Rows.Has(dummy) {
				// We need the actual row to remove it properly (index removal)
				// BTree Get?
				actual, ok := c.Rows.Get(dummy)
				if ok {
					err := c.removeByRow(actual, false)
					if err != nil {
						return err
					}
				}
			}

		case "patch":
			params := cmd.DecodedPayload.(struct {
				I    int
				Diff map[string]interface{}
			})

			dummy := &Row{I: params.I}
			actual, ok := c.Rows.Get(dummy)
			if ok {
				err := c.patchByRow(actual, params.Diff, false)
				if err != nil {
					return err
				}
			}

		case "index":
			indexCommand := cmd.DecodedPayload.(*CreateIndexCommand)

			var options interface{}
			switch indexCommand.Type {
			case "map":
				options = &IndexMapOptions{}
				utils.Remarshal(indexCommand.Options, options)
			case "btree":
				options = &IndexBTreeOptions{}
				utils.Remarshal(indexCommand.Options, options)
			}
			err := c.createIndex(indexCommand.Name, options, false)
			if err != nil {
				return err
			}

		case "drop_index":
			dropIndexCommand := cmd.DecodedPayload.(*DropIndexCommand)
			err := c.dropIndex(dropIndexCommand.Name, false)
			if err != nil {
				return err
			}

		case "set_defaults":
			defaults := cmd.DecodedPayload.(map[string]any)
			err := c.setDefaults(defaults, false)
			if err != nil {
				return err
			}
		}
	}

	if err := <-errs; err != nil {
		return err
	}

	return nil
}
