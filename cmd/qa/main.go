package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/danielfay/dreamsofpotential/internal/game"
)

//go:embed presets/*.json
var presetFS embed.FS

func main() {
	preset := flag.String("preset", "", "preset name or file path")
	list := flag.Bool("list", false, "list available presets")
	run := flag.Bool("run", false, "launch the game after writing the save")
	out := flag.String("out", "", "write save to this path instead of the default location")
	flag.Parse()

	if *list {
		if err := listPresets(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *preset == "" {
		fmt.Fprintln(os.Stderr, "usage: qa -preset <name|path> [-run] [-out <path>]")
		fmt.Fprintln(os.Stderr, "       qa -list")
		os.Exit(1)
	}

	data, err := loadPresetData(*preset)
	if err != nil {
		log.Fatalf("loading preset %q: %v", *preset, err)
	}

	var p game.QAPreset
	if err := json.Unmarshal(data, &p); err != nil {
		log.Fatalf("parsing preset: %v", err)
	}

	w, err := game.BuildQAWorld(p)
	if err != nil {
		log.Fatalf("building world: %v", err)
	}

	if *out != "" {
		if err := game.SaveTo(w, *out); err != nil {
			log.Fatalf("writing save to %s: %v", *out, err)
		}
		fmt.Printf("wrote save to %s\n", *out)
	} else {
		if err := game.Save(w); err != nil {
			log.Fatalf("writing save: %v", err)
		}
		fmt.Printf("wrote save for preset %q\n", p.Name)
	}

	if *run {
		cmd := exec.Command("go", "run", ".")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("running game: %v", err)
		}
	}
}

func loadPresetData(nameOrPath string) ([]byte, error) {
	data, err := presetFS.ReadFile("presets/" + nameOrPath + ".json")
	if err == nil {
		return data, nil
	}
	return os.ReadFile(nameOrPath)
}

func listPresets() error {
	return fs.WalkDir(presetFS, "presets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := presetFS.ReadFile(path)
		if err != nil {
			return err
		}
		var p game.QAPreset
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		fmt.Printf("%-20s %s\n", p.Name, p.Description)
		return nil
	})
}
