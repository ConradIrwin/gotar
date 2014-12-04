// package gotar defines a format for self-extracting executables used by gotar.
//
// Format:
// N bytes of decoder app code
// M bytes of gzipped tar file data
// L bytes of json data
// 32 bytes of footer
//
// The footer consists of 4 big-endian 64 byte numbers that let you decode the file:
//
// 1. N  the length of the app
// 2. M  the length of the gzipped tar file
// 3. L  the length of the json data
// 4. 0x00 0x00 0x00 0x00 0xDE 0xF1 0xA7 0xED  an arbitrary signature to identify the file
// type.
//
// It is currently an error for any of these fields to be 0, both tar data and json data
// must be present.
//
// The format is intended for self-extracting executables created by gotar.
package gotar

import (
	"archive/tar"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// The signature of a gotar executable. It looks kind of like DEFLATED if you squint hard.
var SIGNATURE = int64(0xDEF1A7ED)

// Returned when Read() is called on an invalid archive
var ErrInvalidFormat = fmt.Errorf("invalid gotar file")

// The fixed-length archive footer
type footer struct {
	// How long is the decoder app (i.e. where does the tar file start)
	LenDecoder    int64
	// How long is the tar file
	LenTar    int64
	// How long is the JSON metaData
	LenJson   int64
	// Always set to SIGNATURE to allow quick identification of gotar files
	Signature int64
}

// An Archive opened for reading.
type Archive struct {
	file   *readerAt
	footer footer
	tar    *tar.Reader
	lastFile io.Reader
}

// An Archive Writer. Users should always call WriteDecoder, then WriteFile
// then WriteMetaData, then finally Close.
type Writer struct {
	file     io.Writer
	metaData interface{}
	footer   footer
	flate    *flate.Writer
	tar      *tar.Writer
}

// A countingWriter forwards writes to another io.Writer,
// and counts how much was written.
type countingWriter struct {
	file io.Writer
	dest *int64
}

// Write writes to the countingWriters underlying io.Writer
// and updates the counter.
func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.file.Write(b)
	*w.dest += int64(n)
	return n, err
}

// A readerAt implements io.ReaderAt over the top of io.ReadSeeker
type readerAt struct {
	io.ReadSeeker
	lock sync.Mutex
}
// Read into the buffer from the given offset.
func (r *readerAt) ReadAt(p []byte, off int64) (n int, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.Seek(off, os.SEEK_SET)
	return r.Read(p)
}

// NewWriter instantiates a new Writer.
func NewWriter(f io.WriteSeeker) *Writer {
	writer := &Writer{file: f}
	var err error
	writer.flate, err = flate.NewWriter(&countingWriter{writer.file, &writer.footer.LenTar}, flate.DefaultCompression)
	if err != nil {
		panic(err)
	}
	writer.tar = tar.NewWriter(writer.flate)
	return writer
}
// WriteDecoder writes the decoder executable into the archive. It should be
// a binary that will run on the target system.
func (writer *Writer) WriteDecoder(f io.Reader) error {
	if writer.footer.LenDecoder != 0 {
		panic("gotar.Writer WriteDecoder: tried to write two decoders")
	}

	n, err := io.Copy(writer.file, f)
	writer.footer.LenDecoder = n

	return err
}
// WriteFile adds a file into the archive with the path set relativeTo the
// given directory.
// It is an error to call WriteFile before WriteDecoder or after Close.
func (writer *Writer) WriteFile(file *os.File, relativeTo string) error {

	if writer.footer.LenDecoder == 0 || writer.footer.LenJson != 0 {
		panic("gotar.Writer WriteTar: must writeTar exactly once after writing decoder")
	}

	info, err := os.Stat(file.Name())
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = strings.TrimPrefix(file.Name(), relativeTo)
	hdr.Name = strings.TrimPrefix(hdr.Name, "/")

	err = writer.tar.WriteHeader(hdr)
	if err != nil {
		return err
	}
	f, err := os.Open(file.Name())
	if err != nil {
		return err
	}

	n, err := io.Copy(writer.tar, f)
	if err != nil {
		return err
	}
	if n != hdr.Size {
		return fmt.Errorf("File changed while tarring")
	}

	return nil
}
// WriteMetaData adds MetaData to the archive. Any type that
// can be serialized to be JSON may be used.
func (writer *Writer) WriteMetaData(metaData interface{}) {
	writer.metaData = metaData
}
// Close closes the archive and finishes writing the metaData and
// footer. It does not close the underlying io.Writer.
func (writer *Writer) Close() error {
	if writer.footer.LenDecoder == 0 || writer.footer.LenTar == 0 {
		panic("gotar.Writer Close: closed incomplete archive")
	}

	writer.tar.Flush()
	writer.flate.Close()
	err := json.NewEncoder(&countingWriter{writer.file, &writer.footer.LenJson}).Encode(writer.metaData)
	if err != nil {
		return err
	}
	writer.footer.Signature = SIGNATURE

	return binary.Write(writer.file, binary.BigEndian, writer.footer)
}

// Read an archive from a file.
// It is an error to seek or read from the file while it belongs to a gotar.Archive
func Read(f io.ReadSeeker) (*Archive, error) {
	archive := &Archive{file: &readerAt{f, sync.Mutex{}}}

	err := archive.readFooter()

	if err != nil {
		return nil, err
	}

	stream := flate.NewReader(io.NewSectionReader(archive.file, archive.footer.LenDecoder, archive.footer.LenTar))
	archive.tar = tar.NewReader(stream)

	return archive, nil
}

// NextFile reads the next file from the embedded tar file.
// It returns the tar header and the file contents or an error.
// Once the end of the tar file is reached, io.EOF will be returned.
// The caller must read the entire returned Reader before calling this function again
func (archive *Archive) NextFile() (*tar.Header, io.Reader, error) {

	hdr, err := archive.tar.Next()
	if err != nil {
		return nil, nil, err
	}

	if err != nil {
		return nil, nil, err

	}

	return hdr, io.LimitReader(archive.tar, hdr.Size), nil
}

// ReadMetaData decodes the metaData stored within the gotarred archive.
func (archive *Archive) ReadMetaData(m interface{}) error {
	return json.NewDecoder(io.NewSectionReader(archive.file, archive.footer.LenDecoder+archive.footer.LenTar, archive.footer.LenJson)).Decode(m)
}

// readFooter reads the binary footer and checks that it has the correct format
func (archive *Archive) readFooter() error {
	check, err := archive.file.Seek(-int64(binary.Size(archive.footer)), os.SEEK_END)
	if err != nil {
		return ErrInvalidFormat
	}

	err = binary.Read(archive.file, binary.BigEndian, &archive.footer)
	if err != nil {
		return ErrInvalidFormat
	}

	if archive.footer.Signature != SIGNATURE {
		return ErrInvalidFormat
	}

	if check != archive.footer.LenDecoder+archive.footer.LenTar+archive.footer.LenJson {
		return ErrInvalidFormat
	}

	return nil
}
