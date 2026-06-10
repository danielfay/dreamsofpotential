//go:build js

package game

import (
	"encoding/json"
	"syscall/js"
)

// exportSaveDialog triggers a browser download of the world JSON.
func exportSaveDialog(w *World) {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return
	}

	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)

	parts := js.Global().Get("Array").New()
	parts.Call("push", buf)

	opts := js.Global().Get("Object").New()
	opts.Set("type", "application/json")

	blob := js.Global().Get("Blob").New(parts, opts)
	url := js.Global().Get("URL").Call("createObjectURL", blob)

	a := js.Global().Get("document").Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", "dreamsofpotential-save.json")
	a.Set("style", "display:none")
	js.Global().Get("document").Get("body").Call("appendChild", a)
	a.Call("click")
	js.Global().Get("document").Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

// importSaveDialog opens a hidden <input type="file">, reads the selected JSON,
// validates it, and sends the world to g.importCh.
func importSaveDialog(g *Game) {
	input := js.Global().Get("document").Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".json")
	input.Set("style", "display:none")
	js.Global().Get("document").Get("body").Call("appendChild", input)

	var changeCb js.Func
	changeCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer js.Global().Get("document").Get("body").Call("removeChild", input)
		defer changeCb.Release()

		files := input.Get("files")
		if files.Length() == 0 {
			return nil
		}

		reader := js.Global().Get("FileReader").New()
		var loadCb js.Func
		loadCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
			defer loadCb.Release()
			raw := []byte(reader.Get("result").String())
			var world World
			if err := json.Unmarshal(raw, &world); err != nil {
				return nil
			}
			if world.Version != SaveVersion {
				return nil
			}
			g.importCh <- &world
			return nil
		})
		reader.Set("onload", loadCb)
		reader.Call("readAsText", files.Index(0))
		return nil
	})
	input.Set("onchange", changeCb)
	input.Call("click")
}
