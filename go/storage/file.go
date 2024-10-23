package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sparrowhawktech/toolkit/util"
	"sync"
)

// FileStore
// This is intended for simple, write ahead only storage. Just an organized wrapper over a R/W file
type FileStore struct {
	fileSpec  *string
	mux       *sync.Mutex
	rowBuffer *bytes.Buffer
	count     int
	f         *os.File
	absSpec   *string
	int32Buf  []byte
}

func (o *FileStore) Open() {
	o.mux.Lock()
	defer o.mux.Unlock()
	abs, err := filepath.Abs(*o.fileSpec)
	util.CheckErr(err)
	o.absSpec = &abs
	if !util.FileExists(*o.absSpec) {
		o.initFile()
	}
	o.doOpen()
}

func (o *FileStore) doOpen() {
	f, e := os.OpenFile(*o.absSpec, os.O_RDWR, 0644)
	util.CheckErr(e)
	util.SeekHead(f, 0)
	count := make([]byte, 4)
	util.Read(f, count)
	o.count = int(binary.LittleEndian.Uint32(count))
	o.f = f
}

func (o *FileStore) Close() {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.doClose()
}

func (o *FileStore) doClose() {
	util.CheckErr(o.f.Sync())
	o.closeFile(o.f)
	o.f = nil
}

func (o *FileStore) Count() int {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.count
}

func (o *FileStore) StartRead() int {
	o.mux.Lock()
	util.SeekHead(o.f, 4)
	return o.count
}

func (o *FileStore) ReadNext() *bytes.Buffer {
	util.Read(o.f, o.int32Buf)
	l := int(binary.LittleEndian.Uint32(o.int32Buf))

	o.rowBuffer.Reset()
	cap := o.rowBuffer.Cap()
	if cap < l {
		o.rowBuffer.Grow(l - cap)
	}
	w, err := io.CopyN(o.rowBuffer, o.f, int64(l))
	util.CheckErr(err)
	if w != int64(l) {
		panic(fmt.Sprintf("written length does not match: %d vs %d", l, w))
	}
	return o.rowBuffer
}

func (o *FileStore) EndRead() {
	defer o.mux.Unlock()
}

func (o *FileStore) StartRow() io.Writer {
	o.mux.Lock()
	o.rowBuffer.Reset()
	return o.rowBuffer
}

func (o *FileStore) EndRow() {
	defer o.mux.Unlock()
	binary.LittleEndian.PutUint32(o.int32Buf, uint32(o.count+1))
	util.SeekHead(o.f, 0)
	util.Write(o.f, o.int32Buf)
	o.doStoreRow(o.f)
	o.count++
}

func (o *FileStore) doStoreRow(f *os.File) {
	l := o.rowBuffer.Len()
	binary.LittleEndian.PutUint32(o.int32Buf, uint32(l))
	util.SeekTail(f, 0)
	util.Write(f, o.int32Buf)
	util.Write(f, o.rowBuffer.Bytes())
}

func (o *FileStore) InsertCorrupted() {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.count++
	util.SeekHead(o.f, 0)
	binary.LittleEndian.PutUint32(o.int32Buf, uint32(o.count))
	util.Write(o.f, o.int32Buf)
	util.SeekTail(o.f, 0)
	binary.LittleEndian.PutUint32(o.int32Buf, uint32(100))
	util.Write(o.f, o.int32Buf)
	util.Write(o.f, o.int32Buf)

}

func (o *FileStore) closeFile(f *os.File) {
	err := f.Close()
	util.CheckErr(err)
}

func (o *FileStore) initFile() {
	f, err := os.Create(*o.absSpec)
	util.CheckErr(err)
	defer o.closeFile(f)
	count := []byte{0, 0, 0, 0}
	util.Write(f, count)
}

func (o *FileStore) Vacuum(maxRows int) {
	o.mux.Lock()
	defer o.mux.Unlock()
	tmpSpec := o.createVacuumed(maxRows)
	o.doClose()
	err := os.Rename(tmpSpec, *o.absSpec)
	util.CheckErr(err)
	o.doOpen()
}

func (o *FileStore) createVacuumed(maxRows int) string {
	toRemove := o.count - maxRows
	if toRemove < 0 {
		toRemove = o.count
	}
	abs, err := filepath.Abs(*o.absSpec + ".tmp")
	util.CheckErr(err)
	if util.FileExists(abs) {
		err := os.Remove(abs)
		util.CheckErr(err)
	}
	tmp, err := os.Create(abs) //we don't to play smart and risk corrupting the file
	util.CheckErr(err)
	defer func() { util.CheckErr(tmp.Close()) }()
	binary.LittleEndian.PutUint32(o.int32Buf, 0)
	util.Write(tmp, o.int32Buf)
	util.SeekHead(o.f, 4)
	n := uint32(0)
	for i := 0; i < o.count; i++ {
		good := o.safeLoadRow()
		if i >= toRemove && good {
			o.doStoreRow(tmp)
			n++
		}
	}
	util.SeekHead(tmp, 0)
	binary.LittleEndian.PutUint32(o.int32Buf, n)
	util.Write(tmp, o.int32Buf)
	util.CheckErr(tmp.Sync())
	return abs
}

func (o *FileStore) safeLoadRow() (result bool) {
	defer func() {
		if r := recover(); r != nil {
			util.ProcessError(r)
			result = false
		}
	}()
	o.ReadNext()
	return true
}

func NewFileStore(fileSpec string) *FileStore {
	return &FileStore{fileSpec: &fileSpec, mux: &sync.Mutex{}, rowBuffer: &bytes.Buffer{}, int32Buf: make([]byte, 4)}
}
