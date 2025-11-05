package rdmutex

import (
	"errors"
	"sync"
	"sync/atomic"
)

// RDMutex — это мьютекс, который переводит нового писателя в режим читателя, если уже есть активный писатель.
type RDMutex struct {
	mu          sync.Mutex
	writer      int32
	writerCond  *sync.Cond
	readerCond  *sync.Cond
	readerCount int32
}

// NewRDMutex создаёт новый экземпляр RDMutex.
func NewRDMutex() *RDMutex {
	dm := &RDMutex{}
	dm.writerCond = sync.NewCond(&dm.mu)
	dm.readerCond = sync.NewCond(&dm.mu)
	return dm
}

// Lock пытается захватить мьютекс в роли писателя. Если уже есть активный писатель,
// вызывающий поток становится читателем и возвращается false. В противном случае
// вызывающий поток становится писателем и возвращается true.
func (dm *RDMutex) Lock() bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if atomic.LoadInt32(&dm.writer) == 1 {
		atomic.AddInt32(&dm.readerCount, 1)
		return false
	}

	atomic.StoreInt32(&dm.writer, 1)
	for atomic.LoadInt32(&dm.readerCount) != 0 {
		dm.readerCond.Wait()
	}
	return true
}

// Wait ожидает, пока активный писатель не освободит мьютекс. Должен вызываться
// только читателем (после того, как Lock вернул false).
func (dm *RDMutex) Wait() {
	dm.mu.Lock()
	for atomic.LoadInt32(&dm.writer) == 1 {
		dm.writerCond.Wait()
	}
	dm.mu.Unlock()
}

// Unlock освобождает мьютекс, если вызывающий поток является писателем.
// Возвращает ошибку, если мьютекс не заблокирован писателем.
func (dm *RDMutex) Unlock() error {
	dm.mu.Lock()

	if atomic.LoadInt32(&dm.writer) != 1 {
		dm.mu.Unlock()
		return errors.New("разблокировка незаблокированного мьютекса")
	}

	atomic.StoreInt32(&dm.writer, 0)

	dm.mu.Unlock()

	dm.writerCond.Broadcast()
	return nil
}

// RLock захватывает мьютекс в роли читателя, ожидая завершения работы активного писателя.
func (dm *RDMutex) RLock() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	for atomic.LoadInt32(&dm.writer) == 1 {
		dm.writerCond.Wait()
	}
	atomic.AddInt32(&dm.readerCount, 1)
}

// RUnlock освобождает мьютекс для читателя. Уведомляет писателя, если читателей больше нет.
// Возвращает ошибку, если нет читателей для разблокировки.
func (dm *RDMutex) RUnlock() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if atomic.LoadInt32(&dm.readerCount) <= 0 {
		return errors.New("разблокировка незаблокированного мьютекса")
	}
	atomic.AddInt32(&dm.readerCount, -1)
	if atomic.LoadInt32(&dm.readerCount) == 0 {
		dm.readerCond.Signal()
	}
	return nil
}
