package collectionv2

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/fulldump/inceptiondb/utils"
)

type loadedCommand struct {
	seq            int
	cmd            *Command
	decodedPayload interface{}
	err            error
}

func loadCommands(r io.Reader, concurrency int) (<-chan loadedCommand, <-chan error) {
	out := make(chan loadedCommand, 100)
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
					var decodedPayload interface{}
					if err == nil {
						switch cmd.Name {
						case "insert":
							// For insert, we decode into map[string]interface{}
							// This will be used to populate Row.Decoded
							m := map[string]interface{}{}
							err = json.Unmarshal(cmd.Payload, &m)
							decodedPayload = m
						case "remove":
							params := struct {
								I int
							}{}
							err = json.Unmarshal(cmd.Payload, &params)
							decodedPayload = params
						case "patch":
							params := struct {
								I    int
								Diff map[string]interface{}
							}{}
							err = json.Unmarshal(cmd.Payload, &params)
							decodedPayload = params
						case "index":
							indexCommand := &CreateIndexCommand{}
							err = json.Unmarshal(cmd.Payload, indexCommand)
							decodedPayload = indexCommand
						case "drop_index":
							dropIndexCommand := &DropIndexCommand{}
							err = json.Unmarshal(cmd.Payload, dropIndexCommand)
							decodedPayload = dropIndexCommand
						case "set_defaults":
							defaults := map[string]any{}
							err = json.Unmarshal(cmd.Payload, &defaults)
							decodedPayload = defaults
						}
					}
					results <- loadedCommand{
						seq:            item.seq,
						cmd:            cmd,
						decodedPayload: decodedPayload,
						err:            err,
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
		buffer := map[int]loadedCommand{}
		nextSeq := 0

		for res := range results {
			if res.err != nil {
				errChan <- res.err
				return
			}

			if res.seq == nextSeq {
				out <- res
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
				buffer[res.seq] = res
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
		switch cmd.cmd.Name {
		case "insert":
			// Use decoded payload if available
			row := &Row{
				Payload: cmd.cmd.Payload,
				Decoded: cmd.decodedPayload,
			}
			err := c.addRow(row)
			if err != nil {
				return err
			}
			atomic.AddInt64(&c.Count, 1)
		case "remove":
			params := cmd.decodedPayload.(struct{ I int })
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
			params := cmd.decodedPayload.(struct {
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
			indexCommand := cmd.decodedPayload.(*CreateIndexCommand)

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
			dropIndexCommand := cmd.decodedPayload.(*DropIndexCommand)
			err := c.dropIndex(dropIndexCommand.Name, false)
			if err != nil {
				return err
			}

		case "set_defaults":
			defaults := cmd.decodedPayload.(map[string]any)
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
