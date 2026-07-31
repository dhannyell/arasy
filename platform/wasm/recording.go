//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"
)

// recordingPort is the browser half of the dev panel's recording I/O. The
// panel itself knows nothing about any of this: it hands over bytes to save
// and asks for bytes to load, and everything below — Blob URLs, anchor
// clicks, a hidden file input, a promise — is the browser's answer to those
// two questions.
type recordingPort struct {
	log *slog.Logger

	input js.Value
	// deliver hands loaded bytes to the panel. Loading is asynchronous here:
	// the picker returns immediately and the file arrives frames later, so
	// Load reports nothing and this closure finishes the job.
	deliver func(data []byte, name string)

	// callbacks are retained so the Go GC does not release a js.Func the DOM
	// still holds. A collected handler stops firing silently.
	callbacks []js.Func
}

func newRecordingPort(log *slog.Logger, deliver func(data []byte, name string)) *recordingPort {
	p := &recordingPort{log: log, deliver: deliver}

	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".arrp")
	input.Get("style").Set("display", "none")
	doc.Get("body").Call("appendChild", input)
	p.input = input

	change := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			p.readPicked(args[0].Get("target"))
		}
		return nil
	})
	p.callbacks = append(p.callbacks, change)
	input.Call("addEventListener", "change", change)

	return p
}

// Save downloads the bytes as a file through a Blob URL.
func (p *recordingPort) Save(name string, data []byte) error {
	doc := js.Global().Get("document")
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	blob := js.Global().Get("Blob").New(
		[]any{array},
		map[string]any{"type": "application/octet-stream"},
	)

	url := js.Global().Get("URL").Call("createObjectURL", blob)
	anchor := doc.Call("createElement", "a")
	anchor.Set("href", url)
	anchor.Set("download", name)
	doc.Get("body").Call("appendChild", anchor)
	anchor.Call("click")
	doc.Get("body").Call("removeChild", anchor)
	js.Global().Get("URL").Call("revokeObjectURL", url)
	return nil
}

// Load opens the file picker and reports that nothing is ready yet. The
// browser will not block for a user choosing a file, so the bytes reach the
// panel through deliver instead.
func (p *recordingPort) Load() ([]byte, string, bool, error) {
	// Clearing the value first makes picking the same file twice in a row
	// fire change again, which is what a user re-loading a replay expects.
	p.input.Set("value", "")
	p.input.Call("click")
	return nil, "", false, nil
}

func (p *recordingPort) readPicked(input js.Value) {
	files := input.Get("files")
	if files.Length() == 0 {
		return
	}
	file := files.Index(0)
	name := file.Get("name").String()

	onFulfilled := js.FuncOf(func(this js.Value, args []js.Value) any {
		buffer := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, buffer.Length())
		js.CopyBytesToGo(data, buffer)
		p.deliver(data, name)
		return nil
	})
	onRejected := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.log.Error("read replay file failed", "name", name, "err", args[0].String())
		return nil
	})
	p.callbacks = append(p.callbacks, onFulfilled, onRejected)
	file.Call("arrayBuffer").Call("then", onFulfilled, onRejected)
}
