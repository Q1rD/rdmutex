package mutex

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRDMutex_BasicLockUnlock(t *testing.T) {
	dm := NewRDMutex()

	// Базовый тест на блокировку писателя
	dm.Lock()

	// Проверяем, что writer установлен
	if atomic.LoadInt32(&dm.writer) != 1 {
		t.Error("writer should be 1 after Lock()")
	}

	dm.Unlock()

	// Проверяем, что writer сброшен после разблокировки
	if atomic.LoadInt32(&dm.writer) != 0 {
		t.Error("writer should be 0 after Unlock()")
	}
}

func TestRDMutex_BasicRLockRUnlock(t *testing.T) {
	dm := NewRDMutex()

	// Базовый тест на блокировку читателя
	dm.RLock()

	// Проверяем, что счетчик читателей увеличился
	if atomic.LoadInt32(&dm.readerCount) <= 0 {
		t.Error("readerCount should be positive after RLock()")
	}

	dm.RUnlock()

	// Проверяем, что счетчик читателей сбросился
	if atomic.LoadInt32(&dm.readerCount) != 0 {
		t.Error("readerCount should be 0 after RUnlock()")
	}
}

func TestRDMutex_WriterBlocksSecondWriter(t *testing.T) {
	dm := NewRDMutex()
	var secondWriterBecameReader bool = true

	// Первый писатель блокирует
	if dm.Lock() {
		t.Log("Первый писатель заблокировал мьютекс")
	}

	go func() {
		// Второй писатель должен стать читателем и
		// заблокироваться до разблокировки первого писателя
		if !dm.Lock() {
			t.Log("Второй писатель стал читателем")
			// тк стали читателем насильно, то
			// нужно дождаться старого писателя
			dm.Wait()
			if secondWriterBecameReader != false {
				t.Error("первый писатель сменил значение, но второй не увидел")
			}
			dm.RUnlock()
			return
		}
		// Если мы дошли сюда, значит второй писатель
		// не стал читателем
		secondWriterBecameReader = true
		dm.Unlock()
	}()

	// Даем время горутине попытаться захватить блокировку
	time.Sleep(100 * time.Millisecond)

	// Второй писатель должен был стать читателем
	// пока первый писатель активен
	if secondWriterBecameReader == false {
		t.Error("Second writer should be blocked while first writer is active")
	}

	// Первый писатель меняет значение
	secondWriterBecameReader = false

	// Освобождаем первого писателя
	dm.Unlock()

	// Даем время второй горутине завершиться
	time.Sleep(100 * time.Millisecond)

	// Второй писатель мог сменить на true,
	// если бы не стал читателем насильно
	if secondWriterBecameReader != false {
		t.Error("Second writer should have become reader after first writer released lock")
	}
}

func TestRDMutex_MultipleWritersBecomeReaders(t *testing.T) {
	dm := NewRDMutex()
	var (
		activeWriters int32
		becameReaders int32
		completed     int32
		totalWriters  = 5
	)

	// Первый писатель захватывает мьютекс
	if !dm.Lock() {
		t.Fatal("Первый писатель не смог захватить мьютекс")
	}
	atomic.StoreInt32(&activeWriters, 1)

	// Запускаем несколько писателей
	for i := 0; i < totalWriters; i++ {
		go func(id int) {
			// Попытка захватить как писатель
			if dm.Lock() {
				// Если удалось стать писателем
				atomic.AddInt32(&activeWriters, 1)
				// Имитируем работу писателя
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&activeWriters, -1)
				dm.Unlock()
			} else {
				// Стали читателем насильно
				atomic.AddInt32(&becameReaders, 1)
				// Ждем завершения оригинального писателя
				dm.Wait()
				// Проверяем, что оригинальный писатель завершил работу
				if atomic.LoadInt32(&activeWriters) != 0 {
					t.Errorf("Горутина %d: оригинальный писатель все еще активен", id)
				}
				dm.RUnlock()
			}
			atomic.AddInt32(&completed, 1)
		}(i)
	}

	// Даем время всем горутинам попытаться захватить блокировку
	time.Sleep(200 * time.Millisecond)

	// Проверяем, что только один писатель активен
	if atomic.LoadInt32(&activeWriters) != 1 {
		t.Errorf("Ожидался 1 активный писатель, получили %d", atomic.LoadInt32(&activeWriters))
	}

	// Проверяем, что остальные стали читателями
	expectedReaders := int32(totalWriters)
	if atomic.LoadInt32(&becameReaders) != expectedReaders {
		t.Errorf("Ожидалось %d читателей, получили %d", expectedReaders, atomic.LoadInt32(&becameReaders))
	}

	// Освобождаем первого писателя
	atomic.StoreInt32(&activeWriters, 0)
	dm.Unlock()

	// Ждем завершения всех горутин
	waitStart := time.Now()
	for atomic.LoadInt32(&completed) < int32(totalWriters) {
		if time.Since(waitStart) > 2*time.Second {
			t.Fatal("Таймаут ожидания завершения горутин")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Проверяем, что все завершились
	if atomic.LoadInt32(&completed) != int32(totalWriters) {
		t.Errorf("Завершилось %d горутин из %d", atomic.LoadInt32(&completed), totalWriters)
	}
}
