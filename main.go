package main

import (
	"fmt"
	"sync"
	"time"

	rdmutex "github.com/Q1rD/rdmutex/rdmutex"
)

// SharedResource представляет общий ресурс с счётчиком.
type SharedResource struct {
	count int
	mu    *rdmutex.RDMutex
}

func NewSharedResource() *SharedResource {
	return &SharedResource{
		count: 0,
		mu:    rdmutex.NewRDMutex(),
	}
}

// Update пытается увеличить счётчик как писатель. Если другой писатель активен,
// горутина становится читателем и выводит текущее значение.
func (sr *SharedResource) Update(id int) {
	if sr.mu.Lock() {
		// Захватили мьютекс как писатель
		fmt.Printf("Горутина %d: Захвачен как писатель, увеличиваем счётчик\n", id)
		sr.count++
		time.Sleep(500 * time.Millisecond) // Симуляция работы
		sr.mu.Unlock()
	} else {
		// Стали читателем из-за активного писателя
		fmt.Printf("Горутина %d: Стал читателем, ждём писателя\n", id)
		sr.mu.Wait()
		fmt.Printf("Горутина %d: Прочитано значение счётчика: %d\n", id, sr.count)
		if err := sr.mu.RUnlock(); err != nil {
			fmt.Printf("Горутина %d: Ошибка разблокировки: %v\n", id, err)
		}
	}
}

// Read читает текущее значение счётчика как читатель.
func (sr *SharedResource) Read(id int) {
	sr.mu.RLock()
	fmt.Printf("Горутина %d: Чтение, текущее значение счётчика: %d\n", id, sr.count)
	time.Sleep(50 * time.Millisecond) // Симуляция чтения
	if err := sr.mu.RUnlock(); err != nil {
		fmt.Printf("Горутина %d: Ошибка разблокировки: %v\n", id, err)
	}
}

func main() {
	sr := NewSharedResource()
	var wg sync.WaitGroup

	// Запускаем 1 писателя
	wg.Add(1)
	go func() {
		defer wg.Done()
		sr.Update(1)
	}()

	time.Sleep(50 * time.Millisecond)

	// Запускаем ещё 1 писателя
	wg.Add(1)
	go func() {
		defer wg.Done()
		sr.Update(2)
	}()

	// Запускаем 2 читателя
	for i := 3; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sr.Read(id)
		}(i)
	}

	// Ждём завершения всех горутин
	wg.Wait()
	fmt.Printf("Итоговое значение счётчика: %d\n", sr.count)
}
