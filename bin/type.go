package main

import (
	"io"
	"os"
	"path/filepath"

	"github.com/Velocidex/go-vmdk/parser"
	kingpin "github.com/alecthomas/kingpin/v2"
	ntfs_parser "www.velocidex.com/golang/go-ntfs/parser"
)

var (
	info_command_type = app.Command(
		"type", "Type of a vmdk file.")

	info_command_type_file_arg = info_command_type.Arg(
		"file", "The image file to inspect",
	).Required().String()
)

func doType() {
	fd, err := os.Open(*info_command_type_file_arg)
	kingpin.FatalIfError(err, "Can not open filesystem")

	reader, _ := ntfs_parser.NewPagedReader(
		getReader(fd), 1024, 10000)

	st, err := fd.Stat()
	kingpin.FatalIfError(err, "Can not open filesystem")

	vmdk, err := parser.GetVMDKContext(reader, int(st.Size()),
		func(filename string) (reader io.ReaderAt, closer func(), err error) {
			full_path := filepath.Join(
				filepath.Dir(*info_command_type_file_arg), filename)
			fd, err := os.Open(full_path)
			if err != nil {
				return nil, nil, err
			}

			reader, err = ntfs_parser.NewPagedReader(
				getReader(fd), 1024, 10000)
			return reader, func() { fd.Close() }, nil
		})
	kingpin.FatalIfError(err, "Can not open filesystem")

	config := vmdk.Config()
	parser.PrintVMDKConfig(*config)
}

func init() {
	command_handlers = append(command_handlers, func(command string) bool {
		switch command {
		case info_command_type.FullCommand():
			doType()
		default:
			return false
		}
		return true
	})
}
