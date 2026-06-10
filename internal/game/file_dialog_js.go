//go:build js

package game

import (
	"encoding/json"
	"syscall/js"
	"time"
)

// exportSaveDialog triggers a browser download of the world JSON.
// No dialogOpen needed: the download fires instantly from the game's perspective.
func exportSaveDialog(g *Game) {
	data, err := json.MarshalIndent(g.world, "", "  ")
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
	a.Set("download", time.Now().Format("dreams-2006-01-02-15-04.json"))
	a.Set("style", "display:none")
	js.Global().Get("document").Get("body").Call("appendChild", a)
	a.Call("click")
	js.Global().Get("document").Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

// importSaveDialog opens a hidden <input type="file">, reads the selected JSON,
// validates it, and sends the world to g.importCh.
// Sets dialogOpen while the picker is open; cleared on file selected or cancelled.
func importSaveDialog(g *Game) {
	input := js.Global().Get("document").Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".json")
	input.Set("style", "display:none")
	js.Global().Get("document").Get("body").Call("appendChild", input)

	cleanup := func() {
		js.Global().Get("document").Get("body").Call("removeChild", input)
		g.dialogOpen.Store(false)
	}

	var changeCb js.Func
	changeCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer changeCb.Release()
		defer cleanup()

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

	// oncancel fires when the user dismisses the picker without selecting a file.
	// Supported in Chrome 113+ and Firefox 112+.
	var cancelCb js.Func
	cancelCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer cancelCb.Release()
		defer cleanup()
		return nil
	})

	input.Set("onchange", changeCb)
	input.Set("oncancel", cancelCb)
	g.dialogOpen.Store(true)
	input.Call("click")
}
