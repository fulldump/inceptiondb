package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"runtime"
	"strconv"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func decodePayload(cmd *Command) (interface{}, error) {
	switch cmd.Name {
	case "insert":
		m := map[string]interface{}{}
		err := json.Unmarshal(cmd.Payload, &m)
		return m, err
	case "remove":
		params := struct{ I int }{}
		err := json.Unmarshal(cmd.Payload, &params)
		return params, err
	case "patch":
		params := struct {
			I    int
			Diff map[string]interface{}
		}{}
		err := json.Unmarshal(cmd.Payload, &params)
		return params, err
	case "index":
		indexCommand := &CreateIndexCommand{}
		err := json.Unmarshal(cmd.Payload, indexCommand)
		return indexCommand, err
	case "drop_index":
		dropIndexCommand := &DropIndexCommand{}
		err := json.Unmarshal(cmd.Payload, dropIndexCommand)
		return dropIndexCommand, err
	case "set_defaults":
		defaults := map[string]any{}
		err := json.Unmarshal(cmd.Payload, &defaults)
		return defaults, err
	default:
		return nil, nil
	}
}

func encodeCommandToBuffer(command *Command) *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	buf.WriteString(`{"name":"`)
	buf.WriteString(command.Name)
	buf.WriteString(`","uuid":"`)
	buf.WriteString(command.Uuid)
	buf.WriteString(`","timestamp":`)
	buf.WriteString(strconv.FormatInt(command.Timestamp, 10))
	buf.WriteString(`,"start_byte":`)
	buf.WriteString(strconv.FormatInt(command.StartByte, 10))
	buf.WriteString(`,"payload":`)
	if len(command.Payload) > 0 {
		buf.Write(command.Payload)
	} else {
		buf.WriteString(`null`)
	}
	buf.WriteString("}\n")

	return buf
}

func loadJSONCommands(reader io.Reader) (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		concurrency := runtime.NumCPU()

		scanner := bufio.NewScanner(reader)
		const maxCapacity = 16 * 1024 * 1024
		buf := make([]byte, maxCapacity)
		scanner.Buffer(buf, maxCapacity)

		lines := make(chan struct {
			seq  int
			data []byte
		}, 100)

		results := make(chan LoadedCommand, 100)

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
						decodedPayload, err = decodePayload(cmd)
					}

					results <- LoadedCommand{
						Seq:            item.seq,
						Cmd:            cmd,
						DecodedPayload: decodedPayload,
						Err:            err,
					}
				}
			}()
		}

		go func() {
			seq := 0
			for scanner.Scan() {
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
				results <- LoadedCommand{Seq: -1, Err: err}
			}
			wg.Wait()
			close(results)
		}()

		buffer := map[int]LoadedCommand{}
		nextSeq := 0

		for res := range results {
			if res.Err != nil {
				errChan <- res.Err
				return
			}

			if res.Seq == nextSeq {
				out <- res
				nextSeq++

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
				buffer[res.Seq] = res
			}
		}
	}()

	return out, errChan
}
