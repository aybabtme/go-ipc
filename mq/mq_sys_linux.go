// Copyright 2015 Aleksandr Demakin. All rights reserved.

package mq

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/aybabtme/go-ipc/internal/allocator"

	"golang.org/x/sys/unix"
)

const (
	cSIGEV_SIGNAL      = 0
	cSIGEV_NONE        = 1
	cSIGEV_THREAD      = 2
	cNOTIFY_COOKIE_LEN = 32
)

func initLinuxMqNotifications(ch chan<- int) (notifySocket int, cancelSocket int, err error) {
	notifySocket, cancelSocket = -1, -1
	defer func() {
		if err != nil {
			if notifySocket >= 0 {
				unix.Close(notifySocket)
			}
			if cancelSocket >= 0 {
				unix.Close(cancelSocket)
			}
			notifySocket, cancelSocket = -1, -1
		}
	}()
	notifySocket, err = unix.Socket(unix.AF_NETLINK,
		unix.SOCK_RAW|unix.SOCK_CLOEXEC,
		unix.NETLINK_ROUTE)
	if err != nil {
		return
	}
	if cancelSocket, err = unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0); err != nil {
		return
	}
	sockName := linuxMqNotifySocketAddr(cancelSocket)
	unix.Unlink(sockName)
	if err = unix.Bind(cancelSocket, &unix.SockaddrUnix{Name: sockName}); err != nil {
		return
	}
	if err = unix.Listen(cancelSocket, 1); err == nil {
		go listenLinuxMqNotifications(ch, notifySocket, cancelSocket)
	}
	return
}

func listenLinuxMqNotifications(ch chan<- int, notifySocket int, cancelSocket int) {
	var data [cNOTIFY_COOKIE_LEN]byte
	r := &unix.FdSet{}
	defer func() {
		unix.Close(notifySocket)
		unix.Close(cancelSocket)
	}()
	for {
		fdZero(r)
		fdSet(r, notifySocket)
		fdSet(r, cancelSocket)
		_, err := unix.Select(cancelSocket+1, r, nil, nil, nil)
		if err != nil {
			return
		}
		if fdIsSet(r, cancelSocket) {
			return
		} else if fdIsSet(r, notifySocket) {
			n, _, err := unix.Recvfrom(notifySocket, data[:], unix.MSG_NOSIGNAL|unix.MSG_WAITALL)
			if n == cNOTIFY_COOKIE_LEN && err == nil {
				ndata := (*notify_data)(allocator.ByteSliceData(data[:]))
				ch <- ndata.mq_id
			}
		}
	}
}

func cancelLinuxMqNotifications(cancelSocket int) error {
	socket, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	addr := &unix.SockaddrUnix{Name: linuxMqNotifySocketAddr(cancelSocket)}
	err = unix.Connect(socket, addr)
	unix.Close(socket)
	return err
}

func linuxMqNotifySocketAddr(cancelSocket int) string {
	return fmt.Sprintf("/tmp/%d.%d.socket", os.Getpid(), cancelSocket)
}

// https://github.com/mindreframer/golang-stuff/blob/master/github.com/pebbe/zmq2/examples/udpping1.go

func fdSet(p *unix.FdSet, i int) {
	p.Bits[i/64] |= 1 << (uint(i) % 64)
}

func fdIsSet(p *unix.FdSet, i int) bool {
	return (p.Bits[i/64] & (1 << (uint(i) % 64))) != 0
}

func fdZero(p *unix.FdSet) {
	for i := range p.Bits {
		p.Bits[i] = 0
	}
}

// syscalls

type notify_data struct {
	mq_id   int
	padding [cNOTIFY_COOKIE_LEN - unsafe.Sizeof(int(0))]byte
}

type sigval struct { /* Data passed with notification */
	sigval_ptr uintptr /* A pointer-sized value to match the union size in syscall */
}

type sigevent struct {
	sigev_value             sigval
	sigev_signo             int32
	sigev_notify            int32
	sigev_notify_function   uintptr
	sigev_notify_attributes uintptr
	padding                 [8]int32 // 8 is the maximum padding size
}

func mq_open(name string, flags int, mode uint32, attrs *linuxMqAttr) (int, error) {
	nameBytes, err := unix.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	bytes := unsafe.Pointer(nameBytes)
	attrsP := unsafe.Pointer(attrs)
	id, _, err := unix.Syscall6(unix.SYS_MQ_OPEN,
		uintptr(bytes),
		uintptr(flags),
		uintptr(mode),
		uintptr(attrsP),
		0,
		0)
	allocator.Use(bytes)
	allocator.Use(attrsP)
	if err != syscall.Errno(0) {
		if err == unix.ENOENT || err == unix.EEXIST {
			return -1, &os.PathError{Op: "MQ_OPEN", Path: name, Err: err}
		}
		return -1, os.NewSyscallError("MQ_OPEN", err)
	}
	return int(id), nil
}

func mq_timedsend(id int, data []byte, prio int, timeout *unix.Timespec) error {
	rawData := allocator.ByteSliceData(data)
	timeoutPtr := unsafe.Pointer(timeout)
	_, _, err := unix.Syscall6(unix.SYS_MQ_TIMEDSEND,
		uintptr(id),
		uintptr(rawData),
		uintptr(len(data)),
		uintptr(prio),
		uintptr(timeoutPtr),
		0)
	allocator.Use(rawData)
	allocator.Use(timeoutPtr)
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MQ_TIMEDSEND", err)
	}
	return nil
}

func mq_timedreceive(id int, data []byte, prio *int, timeout *unix.Timespec) (int, int, error) {
	rawData := allocator.ByteSliceData(data)
	timeoutPtr := unsafe.Pointer(timeout)
	prioPtr := unsafe.Pointer(prio)
	msgSize, maxMsgSize, err := unix.Syscall6(unix.SYS_MQ_TIMEDRECEIVE,
		uintptr(id),
		uintptr(rawData),
		uintptr(len(data)),
		uintptr(prioPtr),
		uintptr(timeoutPtr),
		0)
	allocator.Use(rawData)
	allocator.Use(timeoutPtr)
	allocator.Use(prioPtr)
	if err != syscall.Errno(0) {
		return 0, 0, os.NewSyscallError("MQ_TIMEDRECEIVE", err)
	}
	return int(msgSize), int(maxMsgSize), nil
}

func mq_notify(id int, event *sigevent) error {
	eventPtr := unsafe.Pointer(event)
	_, _, err := unix.Syscall(unix.SYS_MQ_NOTIFY, uintptr(id), uintptr(eventPtr), uintptr(0))
	allocator.Use(eventPtr)
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MQ_NOTIFY", err)
	}
	return nil
}

func mq_getsetattr(id int, attrs, oldAttrs *linuxMqAttr) error {
	attrsPtr := unsafe.Pointer(attrs)
	oldAttrsPtr := unsafe.Pointer(oldAttrs)
	_, _, err := unix.Syscall(unix.SYS_MQ_GETSETATTR,
		uintptr(id),
		uintptr(attrsPtr),
		uintptr(oldAttrsPtr))
	allocator.Use(attrsPtr)
	allocator.Use(oldAttrsPtr)
	if err != syscall.Errno(0) {
		return os.NewSyscallError("MQ_GETSETATTR", err)
	}
	return nil
}

func mq_unlink(name string) error {
	nameBytes, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	bytes := unsafe.Pointer(nameBytes)
	_, _, err = unix.Syscall(unix.SYS_MQ_UNLINK, uintptr(bytes), uintptr(0), uintptr(0))
	allocator.Use(bytes)
	if err != syscall.Errno(0) {
		if err == unix.ENOENT {
			return &os.PathError{Op: "MQ_UNLINK", Path: name, Err: err}
		}
		return os.NewSyscallError("MQ_UNLINK", err)
	}
	return nil
}
