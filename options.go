package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/mdsakalu/zmx-session-manager/internal/tui"
)

type options struct {
	version   bool
	exitOnEnd bool
	ui        tui.Options
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{ui: tui.DefaultOptions()}
	fs := flag.NewFlagSet("zsm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: zsm [options]")
		fmt.Fprintln(stderr, "Flags override $XDG_CONFIG_HOME/zsm/config.toml (default ~/.config/zsm/config.toml).")
		fs.PrintDefaults()
	}
	fs.BoolVar(&opts.version, "v", false, "print version and exit")
	fs.BoolVar(&opts.version, "version", false, "print version and exit")
	wrap := fs.Bool("wrap", true, "wrap preview lines")
	exit := fs.Bool("exit-on-end", false, "quit when an attached session ends")
	sort := fs.String("sort", "name:asc", "sort by name, clients, pid, memory or uptime, optionally followed by :asc or :desc")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	order, err := tui.ParseSort(*sort)
	if err != nil || opts.version {
		return opts, err
	}

	opts.readConfig(stderr)
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "sort":
			opts.ui.Sort = order
		case "wrap":
			opts.ui.Wrap = *wrap
		case "exit-on-end":
			opts.exitOnEnd = *exit
		}
	})
	return opts, nil
}

func (o *options) readConfig(stderr io.Writer) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Warning: cannot locate zsm config: %v\n", err)
			return
		}
		dir = filepath.Join(home, ".config")
	}
	path := filepath.Join(dir, "zsm", "config.toml")
	var entries map[string]any
	if _, err := toml.DecodeFile(path, &entries); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "Warning: %s: %v\n", path, err)
		}
		return
	}
	for _, key := range slices.Sorted(maps.Keys(entries)) {
		if err := o.applySetting(key, entries[key]); err != nil {
			fmt.Fprintf(stderr, "Warning: %s: %s: %v\n", path, key, err)
		}
	}
}

func (o *options) applySetting(key string, value any) error {
	switch key {
	case "sort":
		spec, ok := value.(string)
		if !ok {
			return errors.New("expected a sort mode string")
		}
		order, err := tui.ParseSort(spec)
		if err != nil {
			return err
		}
		o.ui.Sort = order
	case "wrap", "exit_on_end":
		b, ok := value.(bool)
		if !ok {
			return errors.New("expected true or false")
		}
		if key == "wrap" {
			o.ui.Wrap = b
		} else {
			o.exitOnEnd = b
		}
	default:
		return errors.New("unknown setting")
	}
	return nil
}
