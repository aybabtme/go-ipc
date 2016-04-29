// Copyright 2016 Aleksandr Demakin. All rights reserved.

package main

import (
	"fmt"
	"os"

	"bitbucket.org/avd/go-ipc/shm"
)

func NewShmObject(name string, mode int, perm os.FileMode, typ string) (shm.SharedMemoryObject, error) {
	switch typ {
	case "default", "":
		return shm.NewMemoryObject(name, mode, perm)
	case "wnm":
		return shm.NewWindowsNativeMemoryObject(name), nil
	default:
		return nil, fmt.Errorf("unknown object type '%s'", typ)
	}
}

func DestroyShmObject(name string, typ string) error {
	switch typ {
	case "default", "":
		return shm.DestroyMemoryObject(name)
	case "wnm":
		return nil
	default:
		return fmt.Errorf("unknown object type '%s'", typ)
	}
}