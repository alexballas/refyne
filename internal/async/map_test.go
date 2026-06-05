package async_test

import (
	"sync"
	"testing"

	"github.com/alexballas/refyne/v2/internal/async"
	"github.com/stretchr/testify/assert"
)

func TestMap_LoadAndStore(t *testing.T) {
	m1 := async.Map[string, int]{}

	m1.Store("1", 1)
	assert.Equal(t, 1, m1.Len())

	num, ok := m1.Load("1")
	assert.Equal(t, 1, num)
	assert.True(t, ok)

	num, ok = m1.Load("2")
	assert.Equal(t, 0, num)
	assert.False(t, ok)

	m2 := async.Map[int, *string]{}

	str := "example"
	m2.Store(0, &str)
	assert.Equal(t, 1, m2.Len())

	strptr, ok := m2.Load(0)
	assert.Equal(t, str, *strptr)
	assert.True(t, ok)

	m2.Store(1, nil)
	assert.Equal(t, 2, m2.Len())

	strptr, ok = m2.Load(1)
	assert.True(t, strptr == nil)
	assert.True(t, ok)

	strptr, ok = m2.Load(3)
	assert.True(t, strptr == nil)
	assert.False(t, ok)
}

func TestMap_ClearAndDelete(t *testing.T) {
	m := async.Map[int, *string]{}

	str := "example"
	m.Store(10, &str)
	assert.Equal(t, 1, m.Len())

	m.Store(11, nil)
	assert.Equal(t, 2, m.Len())

	sum := 0
	m.Range(func(key int, value *string) bool {
		sum += key
		return true
	})
	assert.Equal(t, 21, sum)

	m.Delete(10)
	assert.Equal(t, 1, m.Len())
}

func TestMap_RangeAllowsDelete(t *testing.T) {
	m := async.Map[int, string]{}
	m.Store(10, "ten")
	m.Store(11, "eleven")

	m.Range(func(key int, _ string) bool {
		m.Delete(key)
		return true
	})

	assert.Equal(t, 0, m.Len())
}

func TestMap_CombinedLoad(t *testing.T) {
	m := async.Map[int, *string]{}

	str := "1"
	actual, ok := m.LoadOrStore(1, &str)
	assert.Equal(t, &str, actual)
	assert.False(t, ok)

	actual, ok = m.LoadOrStore(1, nil)
	assert.Equal(t, &str, actual)
	assert.True(t, ok)

	m.Store(1, nil)
	actual, ok = m.LoadOrStore(1, nil)
	assert.True(t, actual == nil)
	assert.True(t, ok)

	actual, ok = m.LoadOrStore(2, nil)
	assert.True(t, actual == nil)
	assert.False(t, ok)

	actual, ok = m.LoadAndDelete(1)
	assert.True(t, actual == nil)
	assert.True(t, ok)

	actual, ok = m.LoadAndDelete(1)
	assert.True(t, actual == nil)
	assert.False(t, ok)

	m.Store(1, &str)
	actual, ok = m.LoadAndDelete(1)
	assert.Equal(t, &str, actual)
	assert.True(t, ok)
}

func TestMap_ConcurrentAccess(t *testing.T) {
	m := async.Map[int, int]{}
	var wg sync.WaitGroup

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				key := id*100 + j
				m.Store(key, j)
				m.Load(key)
				m.Range(func(key, value int) bool {
					return true
				})
				m.LoadOrStore(key, j)
				m.LoadAndDelete(key)
			}
		}(i)
	}

	wg.Wait()
}
