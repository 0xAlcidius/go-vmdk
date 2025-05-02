package parser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	SPARSE_MAGICNUMBER = 0x564d444b
	SECTOR_SIZE        = 512
)

var (
	StartDescriptorRegex   = regexp.MustCompile("^# Disk DescriptorFile")
	StartExtentRegex       = regexp.MustCompile("^# Extent description")
	StartDiskDataBaseRegex = regexp.MustCompile("^# The Disk Data Base")
	ExtentRegex            = regexp.MustCompile(`(RW|R) (\d+) ([A-Z]+) "([^"]+)"(?: (\d+))?`)
)

type VMDKContext struct {
	config  *VMDKConfig
	profile *VMDKProfile
	reader  io.ReaderAt

	extents []Extent

	total_size int64
}

func (self *VMDKContext) Size() int64 {
	return self.total_size
}

func (self *VMDKContext) Config() *VMDKConfig {
	return self.config
}

func (self *VMDKContext) Debug() {
	for _, i := range self.extents {
		i.Debug()
	}
}

func (self *VMDKContext) Close() {
	for _, i := range self.extents {
		i.Close()
	}
}

func (self *VMDKContext) getExtentForOffset(offset int64) (
	extent Extent, err error) {

	n := sort.Search(len(self.extents),
		func(i int) bool {
			extent := self.extents[i]
			virtual_offset := extent.VirtualOffset()

			return virtual_offset > offset
		})

	if n < 1 || n > len(self.extents) {
		return nil, io.EOF
	}

	extent = self.extents[n-1]
	virtual_offset := extent.VirtualOffset()
	extent_size := extent.TotalSize()

	// extent starts after offset.
	if virtual_offset > offset ||

		// extent ends before offset
		virtual_offset+extent_size < offset {
		return nil, io.EOF
	}

	return extent, nil
}

func (self *VMDKContext) normalizeExtents() {
	var extents []Extent
	var offset int64

	// Insert Null Extents
	for _, e := range self.extents {
		if e.VirtualOffset() > offset {
			extents = append(extents, &NullExtent{
				SparseExtent: SparseExtent{
					offset:     offset,
					total_size: e.VirtualOffset() - offset,
				},
			})
		}

		extents = append(extents, e)
		offset += e.TotalSize()
	}

	self.extents = extents
}

func (self *VMDKContext) ReadAt(buf []byte, offset int64) (int, error) {
	i := int64(0)
	buf_len := int64(len(buf))

	// First check the offset is valid for the entire file.
	if offset > self.total_size || offset < 0 {
		return 0, io.EOF
	}

	available_length := self.total_size - offset
	if int64(len(buf)) > available_length {
		buf = buf[:available_length]
	}

	// Now add partial reads for each extent
	for i < buf_len {
		extent, err := self.getExtentForOffset(offset + i)
		if err != nil {
			// Missing extent - zero pad it
			for i := 0; i < len(buf); i++ {
				buf[i] = 0
			}
			return len(buf), nil
		}

		index_in_extent := offset + i - extent.VirtualOffset()
		available_length := extent.TotalSize() - index_in_extent

		// Fill as much of the buffer as possible
		to_read := buf_len - i
		if to_read > available_length {
			to_read = available_length
		}

		n, err := extent.ReadAt(buf[i:i+to_read], index_in_extent)
		if err != nil && err != io.EOF {
			return int(i), err
		}

		// No more data available - we cant make more progress.
		if n == 0 {
			break
		}

		i += int64(n)
	}

	return int(i), nil
}

func GetVMDKContext(
	reader io.ReaderAt, size int,
	opener func(filename string) (
		reader io.ReaderAt, closer func(), err error),
) (*VMDKContext, error) {
	config := NewVMDKConfig()
	profile := NewVMDKProfile()
	res := &VMDKContext{
		config:  config,
		profile: profile,
		reader:  reader,
	}

	if size > 64*1024 {
		size = 64 * 1024 // Read the first 64k of the file.
	}

	// Reading file descriptor.
	buf := make([]byte, size)
	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}

	state := ""

	// check if the file descriptor and virtual disk are in the same file.
	if binary.LittleEndian.Uint32(buf[0:4]) == SPARSE_MAGICNUMBER {
		state = "KDMV"
	}

	for _, line := range strings.Split(string(buf[:n]), "\n") {
		line = strings.TrimSpace(line)

		if StartDescriptorRegex.MatchString(line) {
			state = "Descriptor"
			continue
		}

		if StartExtentRegex.MatchString(line) {
			state = "Extents"
			continue
		}

		if StartDiskDataBaseRegex.MatchString(line) {
			state = "DiskDataBase"
			continue
		}

		switch state {

		// Indicates a VMDK file with an embedded descriptor and virtual disk in the same file.
		case "KDMV":
			if strings.Contains(line, StartDescriptorRegex.String()[1:]) {
				state = "Descriptor"
			}
		case "Descriptor":
			saveDescriptorSetting(line, config)
		case "Extents":
			match := ExtentRegex.FindStringSubmatch(line)
			if len(match) > 0 {
				extent_type := match[3]
				extent_filename := match[4]

				// Try to open the extent file.
				reader, closer, err := opener(extent_filename)
				if err != nil {
					return nil, err
				}

				switch extent_type {
				case "SPARSE":
					extent, err := GetSparseExtent(reader)
					if err != nil {
						return nil, fmt.Errorf("While opening %v: %w",
							extent_filename, err)
					}

					extent.offset = res.total_size
					extent.closer = closer
					extent.filename = extent_filename

					res.total_size += extent.total_size

					res.extents = append(res.extents, extent)
				case "FLAT":
					sectors := ParseInt(match[2])
					extent_filename := match[4]
					offsetSectors := ParseInt(match[5])
					extent, err := GetFlatExtent(
						reader,
						extent_filename,
						offsetSectors,
						sectors,
						res.total_size, // virtual offset = where it sits in the virtual disk
						profile,
						closer,
					)
					if err != nil {
						return nil, fmt.Errorf("While opening flat extent %v: %w", extent_filename, err)
					}

					res.total_size += extent.TotalSize()
					res.extents = append(res.extents, extent)
				default:
					return nil, errors.New("Unsupported extent type " + extent_type)
				}

			} else {
				state = ""
			}

		case "DiskDataBase":
			saveDescriptorSetting(line, config)
		}
	}

	res.normalizeExtents()

	return res, nil
}

func saveDescriptorSetting(line string, config *VMDKConfig) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

	setter, ok := VMDKConfigSetters(config)[key]
	if ok {
		setter(val)
	}
}

func ParseInt(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic("Invalid integer in descriptor: " + s)
	}
	return n
}
