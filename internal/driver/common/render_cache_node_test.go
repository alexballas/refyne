package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/widget"
)

// swapWidget gives each renderer it creates the objects and clipping it is set
// to at the time, so a test sees which renderer a walk used.
type swapWidget struct {
	widget.BaseWidget
	objects []fyne.CanvasObject
	clip    bool
}

func newSwapWidget(objects ...fyne.CanvasObject) *swapWidget {
	w := &swapWidget{objects: objects}
	w.ExtendBaseWidget(w)
	return w
}

func (w *swapWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &swapRenderer{objects: w.objects}
	if w.clip {
		return &swapClipRenderer{r}
	}
	return r
}

type swapRenderer struct {
	objects []fyne.CanvasObject
}

func (*swapRenderer) Destroy()                       {}
func (*swapRenderer) Layout(fyne.Size)               {}
func (*swapRenderer) MinSize() fyne.Size             { return fyne.NewSize(10, 10) }
func (r *swapRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (*swapRenderer) Refresh()                       {}

type swapClipRenderer struct {
	*swapRenderer
}

func (*swapClipRenderer) IsClip() {}

// sizedCanvas is the least of a canvas implementation EnsureMinSize needs.
type sizedCanvas struct {
	fyne.Canvas
	content fyne.CanvasObject
}

func (c *sizedCanvas) Content() fyne.CanvasObject { return c.content }
func (c *sizedCanvas) MinSize() fyne.Size         { return c.content.MinSize() }
func (*sizedCanvas) Resize(fyne.Size)             {}
func (*sizedCanvas) Scale() float32               { return 1 }
func (*sizedCanvas) Size() fyne.Size              { return fyne.NewSize(100, 100) }

func walkedObjects(c *Canvas, tree *renderCacheTree) []fyne.CanvasObject {
	var objs []fyne.CanvasObject
	c.walkTree(tree, func(node *RenderCacheNode, _ fyne.Position) {
		objs = append(objs, node.obj)
	}, nil)
	return objs
}

func TestCanvas_walkTreeKeepsRenderer(t *testing.T) {
	test.NewTempApp(t)

	first := canvas.NewRectangle(color.Black)
	w := newSwapWidget(first)
	defer cache.DestroyRenderer(w)
	tree := &renderCacheTree{root: &RenderCacheNode{obj: w}}
	c := &Canvas{}

	assert.Equal(t, []fyne.CanvasObject{w, first}, walkedObjects(c, tree))
	assert.NotEqual(t, cache.RendererRef{}, tree.root.renderer, "the node keeps the renderer")
	kept := tree.root.renderer
	walkedObjects(c, tree)
	assert.Equal(t, kept, tree.root.renderer)

	destroyed := canvas.NewRectangle(color.White)
	w.objects = []fyne.CanvasObject{destroyed}
	cache.DestroyRenderer(w)
	assert.Equal(t, []fyne.CanvasObject{w, destroyed}, walkedObjects(c, tree), "a destroyed renderer is not walked")
	assert.NotEqual(t, kept, tree.root.renderer)

	cleaned := canvas.NewCircle(color.Black)
	w.objects = []fyne.CanvasObject{cleaned}
	other := &dummyCanvas{}
	cache.SetCanvasForObject(w, other, nil)
	cache.CleanCanvas(other)
	assert.Equal(t, []fyne.CanvasObject{w, cleaned}, walkedObjects(c, tree), "a renderer cleaned with its canvas is not walked")
}

func TestRenderCacheNode_IsClip(t *testing.T) {
	test.NewTempApp(t)

	rect := canvas.NewRectangle(color.Black)
	scroll := container.NewScroll(canvas.NewRectangle(color.White))
	w := newSwapWidget(rect, scroll)
	w.clip = true
	defer cache.DestroyRenderer(w)
	tree := &renderCacheTree{root: &RenderCacheNode{obj: w}}
	c := &Canvas{}

	clips := map[fyne.CanvasObject]bool{}
	walk := func() {
		c.walkTree(tree, func(node *RenderCacheNode, _ fyne.Position) {
			clips[node.obj] = node.IsClip()
		}, nil)
	}
	walk()
	assert.True(t, clips[w], "a widget with a clipping renderer clips")
	assert.False(t, clips[rect])
	assert.True(t, clips[scroll], "a scroller clips")

	w.clip = false
	cache.DestroyRenderer(w)
	walk()
	assert.False(t, clips[w], "a new renderer of another type is asked again")

	w.clip = true
	cache.DestroyRenderer(w)
	walk()
	assert.True(t, clips[w])
}

func TestCanvas_EnsureMinSizeAttachesAgain(t *testing.T) {
	test.NewTempApp(t)

	rect := canvas.NewRectangle(color.Black)
	content := container.NewStack(rect)
	impl := &sizedCanvas{content: content}
	c := &Canvas{}
	c.Initialize(impl, func() {})
	c.SetContentTreeAndFocusMgr(content)
	defer cache.CleanCanvas(impl)

	c.EnsureMinSize()
	assert.Same(t, impl, cache.GetCanvasForObject(rect))
	node := c.contentTree.root.firstChild
	assert.Same(t, rect, node.obj)
	assert.NotEqual(t, cache.CanvasRef{}, node.canvas, "the node keeps the registration")

	other := &dummyCanvas{}
	cache.SetCanvasForObject(rect, other, nil)
	c.EnsureMinSize()
	assert.Same(t, impl, cache.GetCanvasForObject(rect), "registered with another canvas, the object registers anew")

	cache.CleanCanvas(impl)
	assert.Nil(t, cache.GetCanvasForObject(rect))
	c.EnsureMinSize()
	assert.Same(t, impl, cache.GetCanvasForObject(rect), "cleaned with its canvas, the object registers anew")
}
