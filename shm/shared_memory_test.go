// Copyright 2015 Aleksandr Demakin. All rights reserved.

package shm

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	testutil "github.com/aybabtme/go-ipc/internal/test"
	"github.com/aybabtme/go-ipc/mmf"

	"github.com/stretchr/testify/assert"
)

const (
	shmProgPath       = "./internal/test/"
	defaultObjectName = "go-ipc-test"
)

var (
	shmTestData  []byte
	shmProgFiles []string
)

func init() {
	shmTestData = make([]byte, 2048+1024)
	for i := range shmTestData {
		shmTestData[i] = byte(i)
	}
	var err error
	shmProgFiles, err = testutil.LocatePackageFiles(shmProgPath)
	if err != nil {
		panic(err)
	}
	if len(shmProgFiles) == 0 {
		panic("no files to test shm")
	}
	for i, name := range shmProgFiles {
		shmProgFiles[i] = shmProgPath + name
	}
}

// Shared memory test program

func argsForShmCreateCommand(name, typ string, size int64) []string {
	return append(shmProgFiles, "-object="+name, "-type="+typ, "create", fmt.Sprintf("%d", size))
}

func argsForShmDestroyCommand(name, typ string) []string {
	return append(shmProgFiles, "-object="+name, "-type="+typ, "destroy")
}

func argsForShmReadCommand(name, typ string, offset int64, length int) []string {
	return append(shmProgFiles, "-object="+name, "-type="+typ, "read", fmt.Sprintf("%d", offset), fmt.Sprintf("%d", length))
}

func argsForShmTestCommand(name, typ string, offset int64, data []byte) []string {
	strBytes := testutil.BytesToString(data)
	return append(shmProgFiles, "-object="+name, "-type="+typ, "test", fmt.Sprintf("%d", offset), strBytes)
}

func argsForShmWriteCommand(name, typ string, offset int64, data []byte) []string {
	strBytes := testutil.BytesToString(data)
	return append(shmProgFiles, "-object="+name, "-type="+typ, "write", fmt.Sprintf("%d", offset), strBytes)
}

func createMemoryRegionSimple(objMode, regionMode int, size int64, offset int64) (*mmf.MemoryRegion, error) {
	object, err := NewMemoryObject(defaultObjectName, objMode, 0666)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := object.Close()
		if err != nil {
			panic(err.Error())
		}
	}()
	if objMode&os.O_CREATE != 0 {
		if err := object.Truncate(size + offset); err != nil {
			return nil, err
		}
	}
	region, err := mmf.NewMemoryRegion(object, regionMode, offset, int(size))
	if err != nil {
		return nil, err
	}
	return region, nil
}

func TestCreateMemoryObject(t *testing.T) {
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	assert.NoError(t, err)
	if assert.NotNil(t, obj) {
		assert.NoError(t, obj.Close())
		assert.Error(t, obj.Close())
		assert.NoError(t, obj.Destroy())
	}
}

func TestOpenMemoryObjectReadonly(t *testing.T) {
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, obj.Destroy())
	}()
	defer obj.Close()
	obj2, err := NewMemoryObject(defaultObjectName, os.O_RDONLY, 0)
	if !assert.NoError(t, err) {
		return
	}
	defer obj2.Close()
}

func TestDestroyMemoryObject(t *testing.T) {
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	assert.NoError(t, err)
	if assert.NotNil(t, obj) {
		if !assert.NoError(t, obj.Destroy()) {
			return
		}
		_, err = NewMemoryObject(defaultObjectName, os.O_RDONLY, 0666)
		assert.Error(t, err)
	}
}

func TestDestroyMemoryObject2(t *testing.T) {
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if assert.NoError(t, err) {
		obj.Close()
		assert.NoError(t, DestroyMemoryObject(defaultObjectName))
	}
}

func TestCreateMemoryRegionExclusive(t *testing.T) {
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if !assert.NoError(t, err) {
		return
	}
	_, err = NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	assert.Error(t, err)
	obj.Destroy()
}

func TestMemoryObjectSize(t *testing.T) {
	a := assert.New(t)
	pageSize := int64(os.Getpagesize())
	if !a.NoError(DestroyMemoryObject(defaultObjectName)) {
		return
	}
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	defer func() {
		a.NoError(obj.Destroy())
	}()
	if !a.NoError(err) {
		return
	}
	if !a.NoError(obj.Truncate(pageSize - 512)) {
		return
	}
	if runtime.GOOS == "darwin" {
		a.Equal(pageSize, obj.Size())
	} else {
		a.Equal(pageSize-512, obj.Size())
		if !a.NoError(obj.Truncate(1000)) {
			return
		}
		a.Equal(int64(1000), obj.Size())
	}
}

func TestMemoryObjectName(t *testing.T) {
	a := assert.New(t)
	obj, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if a.NoError(err) {
		a.Equal(defaultObjectName, obj.Name())
		a.NoError(obj.Destroy())
	}
}

func TestIfRegionIsAliveAferObjectClose(t *testing.T) {
	object, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, DestroyMemoryObject(defaultObjectName))
	}()
	if !assert.NoError(t, object.Truncate(1024)) {
		return
	}
	region, err := mmf.NewMemoryRegion(object, mmf.MEM_READWRITE, 0, 1024)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, region.Close())
	}()
	if !assert.NoError(t, object.Close()) {
		return
	}
	assert.NotPanics(t, func() {
		data := region.Data()
		for i := range data {
			data[i] = byte(i)
		}
	})
}

func TestMemoryObjectCloseOnGc(t *testing.T) {
	if !assert.NoError(t, DestroyMemoryObject(defaultObjectName)) {
		return
	}
	object, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_RDWR, 0666)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, DestroyMemoryObject(defaultObjectName))
	}()
	file := object.file
	object = nil
	// this is to assure, that the finalized was called and that the
	// corresponding file was closed. this test can theoretically fail, and
	// we use several attempts, as it is not guaranteed that the object is garbage-collected
	// after a call to GC()
	for i := 0; i < 5; i++ {
		runtime.GC()
		if int(-1) == int(file.Fd()) {
			return
		}
		time.Sleep(time.Millisecond * 20)
	}
	// TODO(avd) - close() on darwin
	assert.Fail(t, "the memory object was not finalized during the gc cycle")
}

func TestWriteMemoryRegionSameProcess(t *testing.T) {
	a := assert.New(t)
	if !a.NoError(DestroyMemoryObject(defaultObjectName)) {
		return
	}
	region, err := createMemoryRegionSimple(os.O_CREATE|os.O_EXCL|os.O_RDWR, mmf.MEM_READWRITE, int64(len(shmTestData)), 0)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, region.Close())
		assert.NoError(t, DestroyMemoryObject(defaultObjectName))
	}()
	copy(region.Data(), shmTestData)
	assert.NoError(t, region.Flush(false))
	region2, err := createMemoryRegionSimple(os.O_RDONLY, mmf.MEM_READ_ONLY, int64(len(shmTestData)), 0)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, shmTestData, region2.Data())
	assert.NoError(t, region2.Close())
}

func TestWriteMemoryAnotherProcess(t *testing.T) {
	a := assert.New(t)
	if !a.NoError(DestroyMemoryObject(defaultObjectName)) {
		return
	}
	region, err := createMemoryRegionSimple(os.O_CREATE|os.O_EXCL|os.O_RDWR, mmf.MEM_READWRITE, int64(len(shmTestData)), 128)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, region.Close())
		assert.NoError(t, DestroyMemoryObject(defaultObjectName))
	}()
	copy(region.Data(), shmTestData)
	assert.NoError(t, region.Flush(false))
	result := testutil.RunTestApp(argsForShmTestCommand(defaultObjectName, "", 128, shmTestData), nil)
	assert.NoError(t, result.Err)
}

func TestReadMemoryAnotherProcess(t *testing.T) {
	a := assert.New(t)
	if !a.NoError(DestroyMemoryObject(defaultObjectName)) {
		return
	}
	object, err := NewMemoryObject(defaultObjectName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		assert.NoError(t, object.Destroy())
	}()
	if !assert.NoError(t, object.Truncate(int64(len(shmTestData)))) {
		return
	}
	result := testutil.RunTestApp(argsForShmWriteCommand(defaultObjectName, "", 0, shmTestData), nil)
	if !assert.NoError(t, result.Err) {
		t.Log(result.Output)
		return
	}
	region, err := mmf.NewMemoryRegion(object, mmf.MEM_READ_ONLY, 0, len(shmTestData))
	if !assert.NoError(t, err) {
		return
	}
	defer region.Close()
	assert.Equal(t, shmTestData, region.Data())
}

func TestMemoryRegionNorGcedWithUse(t *testing.T) {
	a := assert.New(t)
	if !a.NoError(DestroyMemoryObject("gc-test")) {
		return
	}
	obj, err := NewMemoryObject("gc-test", os.O_CREATE|os.O_RDWR, 0666)
	if !a.NoError(err) {
		return
	}
	if !a.NoError(obj.Truncate(8192)) {
		return
	}
	region, err := mmf.NewMemoryRegion(obj, mmf.MEM_READWRITE, 0, 8192)
	if !a.NoError(err) {
		return
	}
	defer mmf.UseMemoryRegion(region)
	data := region.Data()
	region = nil
	// we can't use assert.NotPanics here, as if the region is gc'ed,
	// we get segmentation fault, which cannot be handled by user code.
	// so, in order for this test to pass, the following code simply
	// must not crash the entire process.
	for i := 0; i < 5; i++ {
		<-time.After(time.Millisecond * 20)
		runtime.GC()
		for j := range data {
			data[i] = byte(j)
		}
	}
}

func TestMemoryRegionReader(t *testing.T) {
	region, err := createMemoryRegionSimple(os.O_CREATE|os.O_RDWR, mmf.MEM_READ_ONLY, 1024, 0)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		region.Close()
		DestroyMemoryObject(defaultObjectName)
	}()
	reader := mmf.NewMemoryRegionReader(region)
	b := make([]byte, 1024)
	read, err := reader.ReadAt(b, 0)
	if !assert.NoError(t, err) || !assert.Equal(t, 1024, read) {
		return
	}
	read, err = reader.ReadAt(b, 1)
	if !assert.Error(t, err) || !assert.Equal(t, 1023, read) {
		return
	}
	b = make([]byte, 2048)
	read, err = reader.ReadAt(b, 0)
	if !assert.Error(t, err) || !assert.Equal(t, 1024, read) {
		return
	}
	read, err = reader.ReadAt(b, 512)
	if !assert.Error(t, err) || !assert.Equal(t, 512, read) {
		return
	}
}

func TestMemoryRegionWriter(t *testing.T) {
	region, err := createMemoryRegionSimple(os.O_CREATE|os.O_RDWR, mmf.MEM_READWRITE, 1024, 0)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		region.Close()
		DestroyMemoryObject(defaultObjectName)
	}()
	writer := mmf.NewMemoryRegionWriter(region)
	b := make([]byte, 1024)
	written, err := writer.WriteAt(b, 0)
	if !assert.NoError(t, err) || !assert.Equal(t, 1024, written) {
		return
	}
	written, err = writer.WriteAt(b, 1)
	if !assert.Error(t, err) || !assert.Equal(t, 1023, written) {
		return
	}
	b = make([]byte, 2048)
	written, err = writer.WriteAt(b, 0)
	if !assert.Error(t, err) || !assert.Equal(t, 1024, written) {
		return
	}
	written, err = writer.WriteAt(b, 512)
	if !assert.Error(t, err) || !assert.Equal(t, 512, written) {
		return
	}
}

func TestMemoryRegionReaderWriter(t *testing.T) {
	a := assert.New(t)
	data := []byte{1, 2, 3, 4, 5, 6}
	region, err := createMemoryRegionSimple(os.O_CREATE|os.O_RDWR, mmf.MEM_READWRITE, 1024, 0)
	if !assert.NoError(t, err) {
		return
	}
	defer func() {
		a.NoError(region.Close())
		a.NoError(DestroyMemoryObject(defaultObjectName))
	}()
	writer := mmf.NewMemoryRegionWriter(region)
	reader := mmf.NewMemoryRegionReader(region)
	n, err := writer.WriteAt(data, 128)
	if !assert.NoError(t, err) || !assert.Equal(t, n, len(data)) {
		return
	}
	actual := make([]byte, len(data))
	n, err = reader.ReadAt(actual, 128)
	if !assert.NoError(t, err) || !assert.Equal(t, n, len(data)) {
		return
	}
	assert.Equal(t, data, actual)
}
