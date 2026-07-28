package vector

import (
	"sync"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestPathCopiesShareFlattenCache(t *testing.T) {
	path := cachedCurvePath(t)
	first, err := path.flatten(0.25)
	if err != nil {
		t.Fatal(err)
	}

	copied := path
	second, err := copied.flatten(0.25)
	if err != nil {
		t.Fatal(err)
	}

	if &first[0].points[0] != &second[0].points[0] {
		t.Fatal("path copy did not reuse cached flattened geometry")
	}
}

func TestFlattenCacheSupportsConcurrentReaders(t *testing.T) {
	path := cachedCurvePath(t)
	const readers = 32

	var wait sync.WaitGroup
	wait.Add(readers)
	for index := 0; index < readers; index++ {
		go func() {
			defer wait.Done()
			contours, err := path.flatten(0.25)
			if err != nil {
				t.Errorf("flatten() error = %v", err)
				return
			}
			if len(contours) != 1 || len(contours[0].points) < 2 {
				t.Errorf("flatten() returned %+v", contours)
			}
		}()
	}
	wait.Wait()
}

func cachedCurvePath(t *testing.T) Path {
	t.Helper()

	var builder Builder
	builder.MoveTo(math2d.Vec2{})
	builder.CubicTo(
		math2d.Vec2{X: 20, Y: -30},
		math2d.Vec2{X: 80, Y: 30},
		math2d.Vec2{X: 100},
	)
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
