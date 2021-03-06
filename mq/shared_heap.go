// Copyright 2016 Aleksandr Demakin. All rights reserved.

package mq

import (
	"container/heap"
	"errors"
	"unsafe"

	"github.com/aybabtme/go-ipc/internal/allocator"
	"github.com/aybabtme/go-ipc/internal/array"
)

type message struct {
	prio int32
	data []byte
}

type sharedHeap struct {
	array *array.SharedArray
}

func newSharedHeap(raw unsafe.Pointer, maxQueueSize, maxMsgSize int) *sharedHeap {
	return &sharedHeap{
		array: array.NewSharedArray(raw, maxQueueSize, maxMsgSize+4),
	}
}

func openSharedHeap(raw unsafe.Pointer) *sharedHeap {
	return &sharedHeap{
		array: array.OpenSharedArray(raw),
	}
}

func (mq *sharedHeap) maxMsgSize() int {
	return mq.array.ElemSize() - 4
}

func (mq *sharedHeap) maxSize() int {
	return mq.array.Cap()
}

func (mq *sharedHeap) at(i int) message {
	data := mq.array.At(i)
	rawData := allocator.ByteSliceData(data)
	return message{prio: *(*int32)(rawData), data: data[4:]}
}

func (mq *sharedHeap) pushMessage(msg *message) {
	heap.Push(mq, msg)
}

func (mq *sharedHeap) popMessage(data []byte) (int, int, error) {
	msg := mq.at(0)
	if len(msg.data) > len(data) {
		return 0, 0, errors.New("the message is too long")
	}
	copy(data, msg.data)
	heap.Pop(mq)
	return len(msg.data), int(msg.prio), nil
}

func (mq *sharedHeap) safeLen() int {
	return mq.array.SafeLen()
}

// sort.Interface

func (mq *sharedHeap) Len() int {
	return mq.array.Len()
}

func (mq *sharedHeap) Less(i, j int) bool {
	if i == j {
		return false
	}
	lhs, rhs := (*int32)(mq.array.AtPointer(i)), (*int32)(mq.array.AtPointer(j))
	// inverse less logic, as we want max-heap.
	return *lhs > *rhs
}

func (mq *sharedHeap) Swap(i, j int) {
	if i != j {
		mq.array.Swap(i, j)
	}
}

// heap.Interface

func (mq *sharedHeap) Push(x interface{}) {
	msg := x.(*message)
	prioData := allocator.ByteSliceFromUnsafePointer(unsafe.Pointer(&msg.prio), 4, 4)
	mq.array.PushBack(prioData, msg.data)
}

func (mq *sharedHeap) Pop() interface{} {
	mq.array.PopBack()
	return nil
}

func calcSharedHeapSize(maxQueueSize, maxMsgSize int) (int, error) {
	if maxQueueSize == 0 || maxMsgSize == 0 {
		return 0, errors.New("queue size cannot be zero")
	}
	return array.CalcSharedArraySize(maxQueueSize, maxMsgSize+4), nil
}

func minHeapSize() int {
	return array.CalcSharedArraySize(0, 0)
}
