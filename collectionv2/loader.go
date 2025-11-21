package collectionv2

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/fulldump/inceptiondb/utils"
)

type loadedCommand struct {
	seq int
	cmd *Command
	err error
}

func loadCommands(r io.Reader, concurrency int) (<-chan *Command, <-chan error) {
	out := make(chan *Command, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		scanner := bufio.NewScanner(r)
		// Increase buffer size for large lines
		const maxCapacity = 16 * 1024 * 1024
		buf := make([]byte, maxCapacity)
		scanner.Buffer(buf, maxCapacity)

		lines := make(chan struct {
			seq  int
			data []byte
		}, 100)

		results := make(chan loadedCommand, 100)

		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range lines {
					cmd := &Command{}
					err := json.Unmarshal(item.data, cmd)
					results <- loadedCommand{
						seq: item.seq,
						cmd: cmd,
						err: err,
					}
				}
			}()
		}

		// Feeder
		go func() {
			seq := 0
			for scanner.Scan() {
				// Copy data because scanner reuses buffer
				data := make([]byte, len(scanner.Bytes()))
				copy(data, scanner.Bytes())
				lines <- struct {
					seq  int
					data []byte
				}{seq, data}
				seq++
			}
			close(lines)
			if err := scanner.Err(); err != nil {
				results <- loadedCommand{seq: -1, err: err}
			}
			wg.Wait()
			close(results)
		}()

		// Re-assembler
		buffer := map[int]*Command{}
		nextSeq := 0

		for res := range results {
			if res.err != nil {
				errChan <- res.err
				return
			}

			if res.seq == nextSeq {
				out <- res.cmd
				nextSeq++

				// Check buffer
				for {
					if cmd, ok := buffer[nextSeq]; ok {
						delete(buffer, nextSeq)
						out <- cmd
						nextSeq++
					} else {
						break
					}
				}
			} else {
				buffer[res.seq] = res.cmd
			}
		}

		// Check if buffer is empty?
		// If results is closed, we are done.
		// If buffer has items left, it means we missed a sequence number?
		// But we assume reliable delivery from workers.
	}()

	return out, errChan
}

func LoadCollection(filename string, c *Collection) error {
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	concurrency := runtime.NumCPU()
	cmds, errs := loadCommands(f, concurrency)

	for cmd := range cmds {
		switch cmd.Name {
		case "insert":
			_, err := c.addRow(cmd.Payload)
			if err != nil {
				return err
			}
		case "remove":
			params := struct {
				I int
			}{}
			json.Unmarshal(cmd.Payload, &params)
			// Find row by I
			// Since we are loading, and I is stable, we can find it.
			// But wait, addRow assigns new I.
			// If we are loading, we should probably respect the I in the log?
			// Or does the log NOT contain I for insert?
			// Original code: addRow assigns I = len(Rows).
			// Log for insert: Payload.
			// Log for remove: I.
			// If we replay, we must ensure I matches.
			// If we use monotonic ID, we must ensure it matches what was logged?
			// But insert log does NOT contain I.
			// So we must deterministically generate I.
			// If we use atomic counter starting at 0, and replay in order, we get same Is.
			// So it should work.

			// However, remove uses I.
			// We need to find the row with that I.
			// BTree is ordered by I. We can search.
			// But Row.I is the key.
			// We can construct a dummy row with that I and search.

			dummy := &Row{I: params.I}
			if c.Rows.Has(dummy) {
				// We need the actual row to remove it properly (index removal)
				// BTree Get?
				actual, ok := c.Rows.Get(dummy)
				if ok {
					c.removeByRow(actual, false)
				}
			}

		case "patch":
			params := struct {
				I    int
				Diff map[string]interface{}
			}{}
			json.Unmarshal(cmd.Payload, &params)

			dummy := &Row{I: params.I}
			actual, ok := c.Rows.Get(dummy)
			if ok {
				c.patchByRow(actual, params.Diff, false)
			}

		case "index":
			indexCommand := &CreateIndexCommand{}
			json.Unmarshal(cmd.Payload, indexCommand)

			var options interface{}
			switch indexCommand.Type {
			case "map":
				options = &IndexMapOptions{}
				utils.Remarshal(indexCommand.Options, options)
			case "btree":
				options = &IndexBTreeOptions{}
				utils.Remarshal(indexCommand.Options, options)
			}
			c.createIndex(indexCommand.Name, options, false)

		case "set_defaults":
			defaults := map[string]any{}
			json.Unmarshal(cmd.Payload, &defaults)
			c.setDefaults(defaults, false)
		}
	}

	if err := <-errs; err != nil {
		return err
	}

	return nil
}
