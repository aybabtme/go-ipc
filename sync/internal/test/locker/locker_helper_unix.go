// Copyright 2016 Aleksandr Demakin. All rights reserved.

// +build linux freebsd darwin

package main

import (
	"fmt"
	"sync"

	ipc_sync "github.com/aybabtme/go-ipc/sync"
)

func createLocker(typ, name string, flag int) (locker sync.Locker, err error) {
	switch typ {
	case "m":
		locker, err = ipc_sync.NewMutex(name, flag, 0666)
	case "msysv":
		locker, err = ipc_sync.NewSemaMutex(name, flag, 0666)
	case "spin":
		locker, err = ipc_sync.NewSpinMutex(name, flag, 0666)
	case "rw":
		locker, err = ipc_sync.NewRWMutex(name, flag, 0666)
	default:
		err = fmt.Errorf("unknown object type %q", typ)
	}
	return
}

func destroyLocker(typ, name string) error {
	switch typ {
	case "m":
		return ipc_sync.DestroyMutex(name)
	case "msysv":
		return ipc_sync.DestroySemaMutex(name)
	case "spin":
		return ipc_sync.DestroySpinMutex(name)
	case "rw":
		return ipc_sync.DestroyRWMutex(name)
	default:
		return fmt.Errorf("unknown object type %q", typ)
	}
}
