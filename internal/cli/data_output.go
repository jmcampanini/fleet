package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// dataOutput holds the format and destination flags of a data workflow
// command.
type dataOutput struct {
	csv       bool
	file      string
	json      bool
	jsonl     bool
	noHeader  bool
	overwrite bool
}

func (o *dataOutput) bind(command *cobra.Command) {
	flags := command.Flags()
	flags.BoolVar(&o.json, "json", false, "Write one JSON report")
	flags.BoolVar(&o.jsonl, "jsonl", false, "Write one JSON object per item, one per line")
	flags.BoolVar(&o.csv, "csv", false, "Write items as CSV rows after a header row")
	flags.BoolVar(&o.noHeader, "no-header", false, "Omit the CSV header row")
	flags.StringVar(&o.file, "output-file", "", "Write the payload to this file instead of stdout")
	flags.BoolVar(&o.overwrite, "overwrite", false, "Replace the --output-file file when it exists")
	command.MarkFlagsMutuallyExclusive("json", "jsonl", "csv")
}

// records reports whether the format writes bare records, which cannot
// state that they are incomplete.
func (o dataOutput) records() bool {
	return o.jsonl || o.csv
}

// validate rejects flag combinations the formats do not support and, before
// any repository work, an existing output file that --overwrite does not
// cover.
func (o dataOutput) validate(cmd *cobra.Command) error {
	changed := cmd.Flags().Changed
	if changed("no-header") && !o.csv {
		return errors.New("--no-header requires --csv")
	}
	if changed("overwrite") && !changed("output-file") {
		return errors.New("--overwrite requires --output-file")
	}
	if !changed("output-file") {
		return nil
	}
	if o.file == "" {
		return errors.New("--output-file requires a nonempty file path")
	}
	if !o.json && !o.records() {
		return errors.New("--output-file requires --json, --jsonl, or --csv")
	}
	if o.overwrite {
		return nil
	}
	if _, err := os.Lstat(o.file); err == nil {
		return fmt.Errorf("output file %q exists; pass --overwrite to replace it", o.file)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check output file %q: %w", o.file, err)
	}
	return nil
}

// write sends a complete payload to stdout, or to the output file.
func (o dataOutput) write(cmd *cobra.Command, payload []byte) error {
	if o.file == "" {
		_, err := cmd.OutOrStdout().Write(payload)
		return err
	}
	return writeOutputFile(o.file, payload, o.overwrite)
}

// writeOutputFile writes payload to a new file beside path and moves it into
// place, so path holds either its earlier state or the complete payload.
// Without overwrite a hard link places it, which fails if path appeared
// since validation; with overwrite a rename replaces path.
func writeOutputFile(path string, payload []byte, overwrite bool) error {
	temp, err := createSibling(path)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", path, err)
	}
	// A rename consumes the temporary name and a link leaves it redundant;
	// failing to remove it cannot change what path holds.
	defer func() { _ = os.Remove(temp.Name()) }()

	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write output file %q: %w", path, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("flush output file %q: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close output file %q: %w", path, err)
	}
	if overwrite {
		err = os.Rename(temp.Name(), path)
	} else {
		err = os.Link(temp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("place output file %q: %w", path, err)
	}
	return nil
}

// createSibling creates an empty, uniquely named file in path's directory.
// Unlike os.CreateTemp, it requests mode 0666, so the umask decides the
// final permissions as it does for a shell redirection.
func createSibling(path string) (*os.File, error) {
	dir, base := filepath.Split(path)
	for {
		name := filepath.Join(dir, "."+base+"."+rand.Text()[:10]+".tmp")
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if !errors.Is(err, fs.ErrExist) {
			return file, err
		}
	}
}
