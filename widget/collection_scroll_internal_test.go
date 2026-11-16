package widget

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/driver"
	"github.com/alexballas/refyne/v2/internal/widget"
	"github.com/alexballas/refyne/v2/test"
)

// collectionScrollCase builds a collection widget with many rows and reports the
// highest row its update callback has been asked to fill. The scroll container no
// longer refreshes itself on every scroll event, so these guard that the collections,
// which hook into the offset change rather than the refresh, still bind the rows
// that scroll into view.
type collectionScrollCase struct {
	name  string
	build func() (obj fyne.CanvasObject, greatest *int)
	// toOffset and toBottom call the collection's own ScrollToOffset and ScrollToBottom
	toOffset func(obj fyne.CanvasObject, y float32)
	toBottom func(obj fyne.CanvasObject)
}

const collectionScrollRows = 1000

func collectionScrollCases() []collectionScrollCase {
	return []collectionScrollCase{
		{
			name: "List",
			build: func() (fyne.CanvasObject, *int) {
				greatest := -1
				l := NewList(
					func() int { return collectionScrollRows },
					func() fyne.CanvasObject { return NewLabel("Template") },
					func(id ListItemID, o fyne.CanvasObject) {
						if id > greatest {
							greatest = id
						}
						o.(*Label).SetText(strconv.Itoa(id))
					},
				)
				return l, &greatest
			},
			toOffset: func(o fyne.CanvasObject, y float32) { o.(*List).ScrollToOffset(y) },
			toBottom: func(o fyne.CanvasObject) { o.(*List).ScrollToBottom() },
		},
		{
			name: "GridWrap",
			build: func() (fyne.CanvasObject, *int) {
				greatest := -1
				g := NewGridWrap(
					func() int { return collectionScrollRows },
					func() fyne.CanvasObject { return NewLabel("Template") },
					func(id GridWrapItemID, o fyne.CanvasObject) {
						if id > greatest {
							greatest = id
						}
						o.(*Label).SetText(strconv.Itoa(id))
					},
				)
				return g, &greatest
			},
			toOffset: func(o fyne.CanvasObject, y float32) { o.(*GridWrap).ScrollToOffset(y) },
			toBottom: func(o fyne.CanvasObject) { o.(*GridWrap).ScrollToBottom() },
		},
		{
			name: "Tree",
			build: func() (fyne.CanvasObject, *int) {
				greatest := -1
				tr := NewTree(
					func(uid TreeNodeID) []TreeNodeID {
						if uid != "" {
							return nil
						}
						ids := make([]TreeNodeID, collectionScrollRows)
						for i := range ids {
							ids[i] = strconv.Itoa(i)
						}
						return ids
					},
					func(uid TreeNodeID) bool { return uid == "" },
					func(bool) fyne.CanvasObject { return NewLabel("Template") },
					func(uid TreeNodeID, _ bool, o fyne.CanvasObject) {
						if id, err := strconv.Atoi(uid); err == nil {
							if id > greatest {
								greatest = id
							}
						}
						o.(*Label).SetText(uid)
					},
				)
				return tr, &greatest
			},
			toOffset: func(o fyne.CanvasObject, y float32) { o.(*Tree).ScrollToOffset(y) },
			toBottom: func(o fyne.CanvasObject) { o.(*Tree).ScrollToBottom() },
		},
		{
			name: "Table",
			build: func() (fyne.CanvasObject, *int) {
				greatest := -1
				tb := NewTable(
					func() (int, int) { return collectionScrollRows, 2 },
					func() fyne.CanvasObject { return NewLabel("Template") },
					func(id TableCellID, o fyne.CanvasObject) {
						if id.Row > greatest {
							greatest = id.Row
						}
						o.(*Label).SetText(strconv.Itoa(id.Row))
					},
				)
				return tb, &greatest
			},
			toOffset: func(o fyne.CanvasObject, y float32) { o.(*Table).ScrollToOffset(fyne.NewPos(0, y)) },
			toBottom: func(o fyne.CanvasObject) { o.(*Table).ScrollToBottom() },
		},
	}
}

func collectionScroller(obj fyne.CanvasObject) *widget.Scroll {
	switch c := obj.(type) {
	case *List:
		return c.scroller
	case *GridWrap:
		return c.scroller
	case *Tree:
		return c.scroller
	case *Table:
		return c.content
	}
	return nil
}

// setupCollectionScroll shows the case's widget in a window and returns it with its
// scroller, the window canvas and the highest row bound after the first layout.
func setupCollectionScroll(t *testing.T, c collectionScrollCase) (fyne.CanvasObject, *widget.Scroll, fyne.Canvas, *int, int) {
	t.Helper()
	obj, greatest := c.build()
	w := test.NewTempWindow(t, obj)
	w.Resize(fyne.NewSize(200, 300))

	scroller := collectionScroller(obj)
	if !assert.NotNil(t, scroller) {
		t.FailNow()
	}
	initial := *greatest
	if !assert.GreaterOrEqual(t, initial, 0, "the first rows must be bound before scrolling") {
		t.FailNow()
	}
	return obj, scroller, w.Canvas(), greatest, initial
}

// canvasPositionOf returns where obj is drawn on the canvas, which is what the
// test input helpers hit test against.
func canvasPositionOf(c fyne.Canvas, obj fyne.CanvasObject) (found fyne.Position) {
	driver.WalkVisibleObjectTree(c.Content(), func(o fyne.CanvasObject, pos, _ fyne.Position, _ fyne.Size) bool {
		if o == obj {
			found = pos
			return true
		}
		return false
	}, nil)
	return found
}

func TestCollections_ScrollBindsNewRows(t *testing.T) {
	test.NewTempApp(t)

	for _, c := range collectionScrollCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Run("wheel", func(t *testing.T) {
				_, scroller, cv, greatest, initial := setupCollectionScroll(t, c)
				pos := canvasPositionOf(cv, scroller).Add(fyne.NewPos(10, 10))

				test.Scroll(cv, pos, 0, -500)

				assert.Positive(t, scroller.Offset.Y)
				assert.Greater(t, *greatest, initial, "rows scrolled into view must be bound")
			})

			t.Run("scroll bar drag", func(t *testing.T) {
				_, scroller, cv, greatest, initial := setupCollectionScroll(t, c)
				// the thumb sits at the top of the bar along the trailing edge
				pos := canvasPositionOf(cv, scroller).Add(fyne.NewPos(scroller.Size().Width-2, 4))

				test.Drag(cv, pos, 0, 100)

				assert.Positive(t, scroller.Offset.Y)
				assert.Greater(t, *greatest, initial, "rows scrolled into view must be bound")
			})

			t.Run("keyboard", func(t *testing.T) {
				obj, scroller, _, greatest, initial := setupCollectionScroll(t, c)
				for i := 0; i < 40; i++ {
					obj.(fyne.Focusable).TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
				}

				assert.Positive(t, scroller.Offset.Y)
				assert.Greater(t, *greatest, initial, "rows scrolled into view must be bound")
			})

			t.Run("ScrollToOffset", func(t *testing.T) {
				obj, scroller, _, greatest, initial := setupCollectionScroll(t, c)

				c.toOffset(obj, 2000)

				assert.Equal(t, float32(2000), scroller.Offset.Y)
				assert.Greater(t, *greatest, initial, "rows scrolled into view must be bound")
			})

			t.Run("ScrollToBottom", func(t *testing.T) {
				obj, scroller, _, greatest, _ := setupCollectionScroll(t, c)

				c.toBottom(obj)

				assert.Positive(t, scroller.Offset.Y)
				assert.Equal(t, collectionScrollRows-1, *greatest, "the last row must be bound")
			})
		})
	}
}
