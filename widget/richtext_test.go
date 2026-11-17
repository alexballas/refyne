package widget

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/internal/widget"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/theme"
)

func richTextRenderTexts(rich fyne.Widget) []*canvas.Text {
	renderer := cache.Renderer(rich).(*textRenderer)
	texts := make([]*canvas.Text, len(renderer.Objects()))
	for i, obj := range renderer.Objects() {
		texts[i] = obj.(*canvas.Text)
	}
	return texts
}

func trailingBoldErrorSegment() *TextSegment {
	return &TextSegment{Style: RichTextStyle{
		Alignment: fyne.TextAlignTrailing,
		ColorName: theme.ColorNameError,
		TextStyle: fyne.TextStyle{Bold: true},
	}}
}

func TestRichText_Hyperlink_Endline(t *testing.T) {
	u, _ := url.Parse("https://github.com/fyne-io/fyne")
	r := NewRichText(
		&TextSegment{Text: "Text", Style: RichTextStyleInline},
		&HyperlinkSegment{Text: "Link", URL: u},
	)
	r.Resize(r.MinSize())
	view := cache.Renderer(r)

	assert.Len(t, view.Objects(), 2)
	assert.Equal(t, view.Objects()[0].Position().Y, view.Objects()[1].Position().Y)   // same baseline
	assert.Greater(t, view.Objects()[1].Position().X, view.Objects()[0].Position().X) // to the right
}

func TestText_Alignment(t *testing.T) {
	seg := trailingBoldErrorSegment()
	seg.Text = "Test"
	text := NewRichText(seg)
	assert.Equal(t, fyne.TextAlignTrailing, test.TempWidgetRenderer(t, text).Objects()[0].(*canvas.Text).Alignment)
}

func TestText_AlignmentInline(t *testing.T) {
	seg1 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignTrailing,
		},
		Text: "Thing 1",
	}
	seg2 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignTrailing,
			Inline:    true,
		},
		Text: "Thing 2",
	}
	seg3 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignTrailing,
			Inline:    true,
		},
		Text: "Thing 3",
	}
	text := NewRichText(seg1, seg2, seg3)
	text.Resize(fyne.NewSize(150, 50))

	r := test.TempWidgetRenderer(t, text)
	obj1 := r.Objects()[0].(*canvas.Text)
	right1 := obj1.Position().X + obj1.Size().Width
	assert.Equal(t, fyne.TextAlignTrailing, obj1.Alignment)
	obj3 := r.Objects()[2].(*canvas.Text)
	right3 := obj3.Position().X + obj3.Size().Width
	assert.Equal(t, fyne.TextAlignTrailing, obj3.Alignment)

	assert.Equal(t, right1, right3)
}

func TestText_AlignmentInlineCenter(t *testing.T) {
	seg1 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignCenter,
		},
		Text: "MMM",
	}
	seg2 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	seg3 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	seg4 := &TextSegment{
		Style: RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			Inline:    true,
		},
		Text: "MMM",
	}
	text := NewRichText(seg1, seg2, seg3, seg4)
	text.Resize(fyne.NewSize(150, 50))

	r := test.TempWidgetRenderer(t, text)
	obj1 := r.Objects()[0].(*canvas.Text)
	left1 := obj1.Position().X
	assert.Equal(t, fyne.TextAlignCenter, obj1.Alignment)
	obj3 := r.Objects()[2].(*canvas.Text)
	left3 := obj3.Position().X
	assert.Equal(t, fyne.TextAlignCenter, obj3.Alignment)

	assert.Equal(t, left1, left3)
}

func TestText_Row(t *testing.T) {
	text := NewRichTextWithText("")
	text.Segments[0].(*TextSegment).Text = "test"
	text.Refresh()

	assert.Nil(t, text.row(-1))
	assert.Nil(t, text.row(1))

	assert.Equal(t, []rune("test"), text.row(0))
}

func TestText_Rows(t *testing.T) {
	text := NewRichTextWithText("test")
	assert.Equal(t, 1, text.rows())
	textSeg := text.Segments[0].(*TextSegment)

	textSeg.Text = "test\ntest"
	text.Refresh()
	assert.Equal(t, 2, text.rows())

	textSeg.Text = "test\ntest\ntest"
	text.Refresh()
	assert.Equal(t, 3, text.rows())

	textSeg.Text = "test\ntest\ntest\n"
	text.Refresh()
	assert.Equal(t, 4, text.rows())

	textSeg.Text = "\n"
	text.Refresh()
	assert.Equal(t, 2, text.rows())
}

func TestText_RowLength(t *testing.T) {
	text := NewRichTextWithText("test")

	rl := text.rowLength(0)
	assert.Equal(t, 4, rl)
	textSeg := text.Segments[0].(*TextSegment)

	textSeg.Text = "test\ntèsts"
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 4, rl)

	rl = text.rowLength(1)
	assert.Equal(t, 5, rl)

	textSeg.Text = ""
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 0, rl)

	textSeg.Text = "\nhello"
	text.Refresh()
	rl = text.rowLength(0)
	assert.Equal(t, 0, rl)

	rl = text.rowLength(1)
	assert.Equal(t, 5, rl)
}

func TestText_Scroll(t *testing.T) {
	text1 := NewRichTextWithText("test1\ntest2")
	text2 := NewRichTextWithText("test1\ntest2")
	text2.Scroll = widget.ScrollBoth

	assert.Less(t, text2.MinSize().Width, text1.MinSize().Width)
	assert.Less(t, text2.MinSize().Height, text1.MinSize().Height)

	text3 := NewRichTextWithText("test1\ntest2")
	text3.Scroll = widget.ScrollVerticalOnly
	assert.Equal(t, text3.MinSize().Width, text1.MinSize().Width)
	assert.Less(t, text3.MinSize().Height, text1.MinSize().Height)

	text4 := NewRichTextWithText("test1\ntest2")
	text4.Scroll = widget.ScrollVerticalOnly
	text4.Wrapping = fyne.TextWrapBreak

	text3.Resize(fyne.NewSize(32, 32))
	text4.Resize(fyne.NewSize(32, 32))
	assert.Less(t, text4.MinSize().Width, text3.MinSize().Width)
	assert.Equal(t, text4.MinSize().Height, text3.MinSize().Height)

	content3 := test.TempWidgetRenderer(t, text3).Objects()[0].(*widget.Scroll).Content
	content4 := test.TempWidgetRenderer(t, text4).Objects()[0].(*widget.Scroll).Content
	assert.Less(t, content4.MinSize().Width, content3.MinSize().Width)
	assert.Greater(t, content4.MinSize().Height, content3.MinSize().Height)
}

func TestText_InsertAt(t *testing.T) {
	type fields struct {
		buffer string
	}
	type args struct {
		pos   int
		runes string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantBuffer string
	}{
		{
			name:   "case_1",
			fields: fields{buffer: "A\n1"},
			args: args{
				pos:   0,
				runes: "\n",
			},
			wantBuffer: "\nA\n1",
		},
		{
			name:   "case_2",
			fields: fields{buffer: "hello\nèé+^#"},
			args: args{
				pos:   5,
				runes: "\naddme",
			},
			wantBuffer: "hello\naddme\nèé+^#",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichTextWithText(tt.fields.buffer)
			text.insertAt(tt.args.pos, []rune(tt.args.runes))
			assert.Equal(t, tt.wantBuffer, text.String())
		})
	}
}

func TestText_Insert(t *testing.T) {
	text := NewRichTextWithText("")
	text.insertAt(0, []rune("a"))
	assert.Equal(t, "a", text.String())
	text.insertAt(1, []rune("\n"))
	assert.Equal(t, "a\n", text.String())
	text.insertAt(2, []rune("b"))
	assert.Equal(t, "a\nb", text.String())
}

func TestText_DeleteFromTo(t *testing.T) {
	type fields struct {
		buffer string
	}
	type args struct {
		lowBound  int
		highBound int
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       string
		wantBuffer string
	}{
		{
			name:   "case_1",
			fields: fields{buffer: "A\n1"},
			args: args{
				lowBound:  0,
				highBound: 1,
			},
			want:       "A",
			wantBuffer: "\n1",
		},
		{
			name:   "case_2",
			fields: fields{buffer: "A\n1"},
			args: args{
				lowBound:  1,
				highBound: 2,
			},
			want:       "\n",
			wantBuffer: "A1",
		},
		{
			name:   "case_3",
			fields: fields{buffer: "A\nè1"},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want:       "\nè",
			wantBuffer: "A1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichTextWithText(tt.fields.buffer)
			got := text.deleteFromTo(tt.args.lowBound, tt.args.highBound)
			assert.Equal(t, tt.want, string(got))
			assert.Equal(t, tt.wantBuffer, text.String())
		})
	}
}

func TestText_DeleteFromTo_Segments(t *testing.T) {
	type args struct {
		lowBound  int
		highBound int
	}
	tests := []struct {
		name         string
		segments     []RichTextSegment
		args         args
		want         string
		wantSegments []RichTextSegment
	}{
		{
			name: "remove begin",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&TextSegment{Text: "1"},
			},
			args: args{
				lowBound:  0,
				highBound: 1,
			},
			want: "A",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "\n"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove end",
			segments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "\n1"},
			},
			args: args{
				lowBound:  1,
				highBound: 2,
			},
			want: "\n",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove both",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&TextSegment{Text: "è1"},
			},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want: "\nè",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
		{
			name: "remove nontext",
			segments: []RichTextSegment{
				&TextSegment{Text: "A\n"},
				&SeparatorSegment{},
				&TextSegment{Text: "B1"},
			},
			args: args{
				lowBound:  1,
				highBound: 3,
			},
			want: "\nB",
			wantSegments: []RichTextSegment{
				&TextSegment{Text: "A"},
				&TextSegment{Text: "1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := NewRichText(tt.segments...)
			got := text.deleteFromTo(tt.args.lowBound, tt.args.highBound)
			assert.Equal(t, tt.want, string(got))
			for _, s := range tt.wantSegments {
				if txt, ok := s.(*TextSegment); ok {
					txt.parent = text
				}
			}
			assert.Equal(t, tt.wantSegments, text.Segments)
		})
	}
}

func TestText_Multiline(t *testing.T) {
	text := NewRichText(
		&TextSegment{Text: "line1\nli", Style: RichTextStyleStrong},
		&TextSegment{Text: "ne2\nline3", Style: RichTextStyleInline},
	)

	w := test.NewTempWindow(t, text)
	w.Resize(fyne.NewSize(64, 90))
	test.AssertImageMatches(t, "richtext/richtext_multiline.png", w.Canvas().Capture())
}

func TestText_Color(t *testing.T) {
	text := NewRichText(trailingBoldErrorSegment())

	assert.Equal(t, theme.Color(theme.ColorNameError), richTextRenderTexts(text)[0].Color)
}

func TestTextRenderer_ApplyTheme(t *testing.T) {
	label := NewLabel("Test\nLine2")
	texts := labelTextRenderTexts(label)

	text1 := texts[0]
	text2 := texts[1]
	textSize1 := text1.TextSize
	textSize2 := text2.TextSize
	customTextSize1 := textSize1
	customTextSize2 := textSize2
	test.WithTestTheme(t, func() {
		label.Refresh()
		text1 := texts[0]
		text2 := texts[1]
		customTextSize1 = text1.TextSize
		customTextSize2 = text2.TextSize
	})

	assert.NotEqual(t, textSize1, customTextSize1)
	assert.NotEqual(t, textSize2, customTextSize2)
}

func TestTextProvider_LineSizeToColumn(t *testing.T) {
	label := NewLabel("Test")
	label.CreateRenderer() // TODO make this a simple refresh call once it's in
	provider := label.provider

	inPad := theme.InnerPadding()
	textSize := theme.TextSize()
	fullSize := provider.lineSizeToColumn(4, 0, textSize, inPad)
	assert.Equal(t, fullSize, provider.lineSizeToColumn(10, 0, textSize, inPad))
	assert.Greater(t, fullSize.Width, provider.lineSizeToColumn(2, 0, textSize, inPad).Width)

	out := provider.lineSizeToColumn(-1, -1, textSize, inPad)
	assert.Equal(t, out, provider.lineSizeToColumn(0, 0, textSize, inPad))
}

func TestText_splitLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [][2]int
	}{
		{
			name: "Empty",
			text: "",
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Single",
			text: "foo",
			want: [][2]int{
				{0, 3},
			},
		},
		{
			name: "Multiple",
			text: "foo\nbar",
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Trailing",
			text: "foo\nbar\n",
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(&TextSegment{Text: tt.text})
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)
			}
		})
	}
}

func TestText_lineBounds(t *testing.T) {
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), 14, fyne.TextStyle{})
	}
	tests := []struct {
		name     string
		text     string
		wrap     fyne.TextWrap
		trunc    fyne.TextTruncation
		want     [][2]int
		ellipses int
	}{
		{
			name: "Empty_WrapOff",
			text: "",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_Truncate",
			text: "",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name:  "Empty_TruncateClip",
			text:  "",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_WrapBreak",
			text: "",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Empty_WrapWord",
			text: "",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 0},
			},
		},
		{
			name: "Single_Short_WrapOff",
			text: "foobar",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Short_Truncate",
			text: "foobar",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name:  "Single_Short_TruncateClip",
			text:  "foobar",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name:  "Single_Short_TruncateEllipsis",
			text:  "foobar",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
			},
			ellipses: 0, // too short to cut
		},
		{
			name: "Single_Short_WrapBreak",
			text: "foobar",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Short_WrapWord",
			text: "foobar",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 6},
			},
		},
		{
			name: "Single_Long_WrapOff",
			text: "foobar foobar",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 13},
			},
		},
		{
			name: "Single_Long_Truncate",
			text: "foobar foobar",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 10},
			},
		},
		{
			name:  "Single_Long_TruncateClip",
			text:  "foobar foobar",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 10},
			},
		},
		{
			name:  "Single_Long_TruncateEllipsis",
			text:  "foobar foobar",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 9},
			},
			ellipses: 1,
		},
		{
			name: "Single_Long_WrapBreak",
			text: "foobar foobar",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 10},
				{10, 13},
			},
		},
		{
			name: "Single_Long_WrapWord",
			text: "foobar foobar",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
			},
		},
		{
			name: "Multiple_Short_WrapOff",
			text: "foo\nbar",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Short_Truncate",
			text: "foo\nbar",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Short_TruncateClip",
			text:  "foo\nbar",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Short_TruncateEllipsis",
			text:  "foo\nbar",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
			ellipses: 0, // too wide
		},
		{
			name: "Multiple_Short_WrapBreak",
			text: "foo\nbar",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Short_WrapWord",
			text: "foo\nbar",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name: "Multiple_Long_WrapOff",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 27},
				{28, 41},
			},
		},
		{
			name: "Multiple_Long_Truncate",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
			},
		},
		{
			name:  "Multiple_Long_TruncateClip",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
			},
		},
		{
			name:  "Multiple_Long_TruncateEllipsis",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 16},
				{28, 37},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Long_WrapBreak",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 27},
				{28, 38},
				{38, 41},
			},
		},
		{
			name: "Multiple_Long_WrapWord",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
				{14, 20},
				{21, 27},
				{28, 34},
				{35, 41},
			},
		},
		{
			name: "Multiple_Contiguous_Long_WrapOff",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 25},
				{26, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Contiguous_Long_Truncate",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 17},
				{26, 36},
				{39, 39},
			},
		},
		{
			name:  "Multiple_Contiguous_Long_TruncateClip",
			text:  "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{26, 36},
				{39, 39},
			},
		},
		{
			name:  "Multiple_Contiguous_Long_TruncateEllipsis",
			text:  "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 15},
				{26, 34},
				{39, 39},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Contiguous_Long_WrapBreak",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 25},
				{26, 36},
				{36, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Contiguous_Long_WrapWord",
			text: "foobar\nfoobarfoobarfoobar\nfoobarfoobar\n",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 25},
				{26, 36},
				{36, 38},
				{39, 39},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapOff",
			text: "foo\nbar\n",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_Truncate",
			text: "foo\nbar\n",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_TruncateClip",
			text:  "foo\nbar\n",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_TruncateEllipsis",
			text:  "foo\nbar\n",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapBreak",
			text: "foo\nbar\n",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name: "Multiple_Trailing_Short_WrapWord",
			text: "foo\nbar\n",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 3},
				{4, 7},
				{8, 8},
			},
		},
		{
			name:  "Multiple_Trailing_Short_WrapTruncateClip",
			text:  "foo\nbar\nbaz",
			wrap:  fyne.TextWrapBreak,
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
		},
		{
			name:  "Multiple_Trailing_Short_WrapTruncateEllipsis",
			text:  "foo\nbar\nbaz",
			wrap:  fyne.TextWrapBreak,
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 7},
			},
			ellipses: 0,
		},
		{
			name: "Multiple_Trailing_Long_WrapOff",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 6},
				{7, 27},
				{28, 41},
				{42, 42},
			},
		},
		{
			name: "Multiple_Trailing_Long_Truncate",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_TruncateClip",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{28, 38},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_TruncateEllipsis",
			text:  "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 6},
				{7, 16},
				{28, 37},
				{42, 42},
			},
			ellipses: 2,
		},
		{
			name: "Multiple_Trailing_Long_WrapBreak",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 6},
				{7, 17},
				{17, 27},
				{28, 38},
				{38, 41},
				{42, 42},
			},
		},
		{
			name: "Multiple_Trailing_Long_WrapWord",
			text: "foobar\nfoobar foobar foobar\nfoobar foobar\n",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 6},
				{7, 13},
				{14, 20},
				{21, 27},
				{28, 34},
				{35, 41},
				{42, 42},
			},
		},
		{
			name:  "Multiple_Trailing_Long_WrapTruncateClip",
			text:  "foo\nfoobar foobar foobar\nbaz",
			wrap:  fyne.TextWrapBreak,
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 3},
				{4, 14},
			},
		},
		{
			name:  "Multiple_Trailing_Long_WrapTruncateEllipsis",
			text:  "foo\nfoobar foobar foobar\nbazzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			wrap:  fyne.TextWrapBreak,
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 14},
				{14, 23},
			},
			ellipses: 1,
		},
		{
			name:  "Multiple_Trailing_Long_WrapWordTruncateEllipsis",
			text:  "foo\nfoobar foobar foobar\nbaz",
			wrap:  fyne.TextWrapWord,
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 3},
				{4, 10},
				{11, 17},
			},
			ellipses: 1,
		},
		{
			name:  "Multi_byte_ellipsis_not_truncated",
			text:  "🪃 234",
			trunc: fyne.TextTruncateEllipsis,
			wrap:  fyne.TextWrapOff,
			want: [][2]int{
				{0, 5},
			},
			ellipses: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ellipses := 0
			richText := NewRichTextWithText(tt.text)
			richText.Wrapping = tt.wrap
			richText.Truncation = tt.trunc
			got, _ := lineBounds(richText, richText.Segments[0], 76, fyne.NewSize(76, 64), measurer)
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)

				if got[i].ellipsis {
					ellipses++
				}
			}
			assert.Equal(t, tt.ellipses, ellipses)
		})
	}
}

func TestText_lineBounds_hyperlinks(t *testing.T) {
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), 14, fyne.TextStyle{})
	}
	tests := []struct {
		name     string
		text     string
		wrap     fyne.TextWrap
		trunc    fyne.TextTruncation
		want     [][2]int
		ellipses int
	}{
		{
			name: "Hyperlink_Truncate",
			text: "this is a long link",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 9},
			},
		},
		{
			name:  "Hyperlink_TruncateClip",
			text:  "this is a long link",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 9},
			},
		},
		{
			name:  "Hyperlink_TruncateEllipsis",
			text:  "this is a long link",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 8},
			},
			ellipses: 1,
		},
		{
			name: "Hyperlink_WrapBreak",
			text: "this is a long link",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 9},
				{9, 17},
			},
		},
		{
			name: "Hyperlink_WrapWord",
			text: "this is a long link",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 9},
				{10, 14},
				{15, 19},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ellipses := 0
			u, _ := url.Parse("https://fyne.io/")
			richText := NewRichText(&HyperlinkSegment{Text: tt.text, URL: u})
			richText.Wrapping = tt.wrap
			richText.Truncation = tt.trunc
			got, _ := lineBounds(richText, richText.Segments[0], 50, fyne.NewSize(50, 64), measurer)
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)

				if got[i].ellipsis {
					ellipses++
				}
			}
			assert.Equal(t, tt.ellipses, ellipses)
		})
	}
}

func TestText_lineBounds_variable_char_width(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		wrap  fyne.TextWrap
		trunc fyne.TextTruncation
		want  [][2]int
	}{
		{
			name: "IM_WrapOff",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: fyne.TextWrapOff,
			want: [][2]int{
				{0, 20},
			},
		},
		{
			name: "IM_Truncate",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: fyne.TextWrap(fyne.TextTruncateClip),
			want: [][2]int{
				{0, 12},
			},
		},
		{
			name:  "IM_TruncateClip",
			text:  "iiiiiiiiiimmmmmmmmmm",
			trunc: fyne.TextTruncateClip,
			want: [][2]int{
				{0, 12},
			},
		},
		{
			name:  "IM_TruncateEllipsis",
			text:  "iiiiiiiiiimmmmmmmmmm",
			trunc: fyne.TextTruncateEllipsis,
			want: [][2]int{
				{0, 9},
			},
		},
		{
			name: "IM_WrapBreak",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: fyne.TextWrapBreak,
			want: [][2]int{
				{0, 12},
				{12, 16},
				{16, 20},
			},
		},
		{
			name: "IM_WrapWord",
			text: "iiiiiiiiiimmmmmmmmmm",
			wrap: fyne.TextWrapWord,
			want: [][2]int{
				{0, 12},
				{12, 16},
				{16, 20},
			},
		},
	}
	textSize := float32(10)
	textStyle := fyne.TextStyle{}
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), textSize, textStyle)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			richText := NewRichTextWithText(tt.text)
			richText.Wrapping = tt.wrap
			richText.Truncation = tt.trunc
			got, _ := lineBounds(richText, richText.Segments[0], 46, fyne.NewSize(46, 184), measurer)
			for i, wantRow := range tt.want {
				assert.Equal(t, wantRow[0], got[i].begin)
				assert.Equal(t, wantRow[1], got[i].end)
			}
		})
	}
}

func TestText_lineBounds_small_firstWidth(t *testing.T) {
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), 14, fyne.TextStyle{})
	}
	richText := NewRichTextWithText("foobar")
	richText.Wrapping = fyne.TextWrapWord
	richText.Truncation = fyne.TextTruncateOff
	got, _ := lineBounds(richText, richText.Segments[0], 0.1, fyne.NewSize(64, 20), measurer)
	assert.Equal(t, 0, got[0].begin)
	assert.Equal(t, 0, got[0].end)
	assert.Equal(t, 0, got[1].begin)
	assert.Equal(t, 6, got[1].end)
}

func TestText_howManyRunesFit(t *testing.T) {
	maxWidth := float32(46)
	textSize := float32(10)
	textStyle := fyne.TextStyle{}
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), textSize, textStyle)
	}
	charWidth := measurer([]rune("z")).Width
	for name, tt := range map[string]struct {
		text string
		want int
	}{
		"IM": {
			text: "iiiiiiiiiimmmmmmmmmm",
			want: 12,
		},
		"Single_Line": {
			text: "foobar foobar",
			want: 9,
		},
		"WH": {
			text: "wwwww hhhhhh",
			want: 6,
		},
		"DS": {
			text: "dddddd sssssss",
			want: 8,
		},
		"DI": {
			text: "dididi dididd",
			want: 10,
		},
		"XW": {
			text: "xwxwxwxw xwxw",
			want: 7,
		},
		"W": {
			text: "WWWWW",
			want: 4,
		},
		"Empty": {
			text: "",
			want: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, howManyRunesFit([]rune(tt.text), maxWidth, charWidth, measurer))
		})
	}
	t.Run("Zero width", func(t *testing.T) {
		assert.Equal(t, 0, howManyRunesFit([]rune("foo"), 0, charWidth, measurer))
	})
	t.Run("Negative width", func(t *testing.T) {
		assert.Equal(t, 0, howManyRunesFit([]rune("foo"), -1, charWidth, measurer))
	})
	t.Run("Zero character width", func(t *testing.T) {
		assert.Equal(t, 0, howManyRunesFit([]rune("foo"), maxWidth, 0, measurer))
	})
	t.Run("Sub-character width", func(t *testing.T) {
		measurer := func(text []rune) fyne.Size {
			return fyne.NewSize(float32(len(text))*10, 10)
		}
		assert.Equal(t, 0, howManyRunesFit([]rune("foo"), 5, 10, measurer))
	})
	t.Run("Zero measured width", func(t *testing.T) {
		measurer := func([]rune) fyne.Size {
			return fyne.NewSize(0, 10)
		}
		assert.Equal(t, 3, howManyRunesFit([]rune("foo"), 5, 10, measurer))
	})
}

func TestText_lineBounds_truncateMiddle(t *testing.T) {
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), 14, fyne.TextStyle{})
	}

	t.Run("short_no_truncation", func(t *testing.T) {
		richText := NewRichTextWithText("foobar")
		richText.Truncation = fyne.TextTruncateMiddle
		got, _ := lineBounds(richText, richText.Segments[0], 200, fyne.NewSize(200, 64), measurer)
		assert.Equal(t, 1, len(got))
		assert.Empty(t, got[0].displayText)
		assert.False(t, got[0].ellipsis)
	})

	t.Run("long_truncates_in_middle", func(t *testing.T) {
		richText := NewRichTextWithText("/home/user/projects/fyne/file.txt")
		richText.Truncation = fyne.TextTruncateMiddle
		got, _ := lineBounds(richText, richText.Segments[0], 76, fyne.NewSize(76, 64), measurer)
		assert.Equal(t, 1, len(got))
		assert.Contains(t, got[0].displayText, "…")
		// displayText starts with original prefix and ends with original suffix
		assert.Equal(t, byte('/'), got[0].displayText[0])
		assert.Equal(t, byte('t'), got[0].displayText[len(got[0].displayText)-1])
	})

	t.Run("wrapped_out_of_height", func(t *testing.T) {
		richText := NewRichTextWithText("foobar foobar foobar")
		richText.Wrapping = fyne.TextWrapWord
		richText.Truncation = fyne.TextTruncateMiddle
		lineHeight := measurer([]rune("z")).Height
		got, _ := lineBounds(richText, richText.Segments[0], 70, fyne.NewSize(70, lineHeight), measurer)
		assert.Equal(t, 1, len(got))
		assert.True(t, got[0].ellipsis, "wrapped text that runs out of height ends with an ellipsis")
		assert.Empty(t, got[0].displayText)
	})

	t.Run("wrapped_no_character_fits", func(t *testing.T) {
		richText := NewRichTextWithText("foobar")
		richText.Wrapping = fyne.TextWrapWord
		richText.Truncation = fyne.TextTruncateMiddle
		got, _ := lineBounds(richText, richText.Segments[0], 1, fyne.NewSize(1, 64), measurer)
		assert.NotEmpty(t, got)
		for _, b := range got {
			assert.True(t, b.ellipsis)
			assert.Equal(t, b.begin, b.end, "no character is drawn before the ellipsis")
		}
	})
}

func TestText_truncateMiddle(t *testing.T) {
	textSize := float32(10)
	textStyle := fyne.TextStyle{}
	measurer := func(text []rune) fyne.Size {
		return fyne.MeasureText(string(text), textSize, textStyle)
	}
	ellipsisWidth := measurer([]rune("…")).Width

	t.Run("empty", func(t *testing.T) {
		prefix, suffix, full := truncateMiddle(nil, 100, measurer)
		assert.Equal(t, 0, prefix)
		assert.Equal(t, 0, suffix)
		assert.True(t, full)
	})

	t.Run("fits_whole", func(t *testing.T) {
		runes := []rune("hello")
		full := measurer(runes).Width
		prefix, suffix, fullOK := truncateMiddle(runes, full+10, measurer)
		assert.True(t, fullOK)
		assert.Equal(t, len(runes), prefix)
		assert.Equal(t, 0, suffix)
	})

	t.Run("does_not_fit_ellipsis", func(t *testing.T) {
		runes := []rune("hello world")
		prefix, suffix, fullOK := truncateMiddle(runes, ellipsisWidth-1, measurer)
		assert.False(t, fullOK)
		assert.Equal(t, 0, prefix)
		assert.Equal(t, 0, suffix)
	})

	t.Run("balanced_split", func(t *testing.T) {
		runes := []rune("aaaaaaaaaa") // 10 a's
		full := measurer(runes).Width
		// allow enough room for roughly half the characters plus ellipsis
		prefix, suffix, fullOK := truncateMiddle(runes, full/2+ellipsisWidth, measurer)
		assert.False(t, fullOK)
		// prefix should be the larger or equal half
		assert.GreaterOrEqual(t, prefix, suffix)
		// resulting candidate must actually fit
		out := append([]rune{}, runes[:prefix]...)
		out = append(out, '…')
		out = append(out, runes[len(runes)-suffix:]...)
		assert.LessOrEqual(t, measurer(out).Width, full/2+ellipsisWidth)
	})
}

// richTextRowTexts returns the text drawn for each text and hyperlink segment of a rich text,
// and the right edge of where each one is drawn.
func richTextRowTexts(t *testing.T, rich *RichText) ([]string, []float32) {
	var texts []string
	var ends []float32
	for _, obj := range test.TempWidgetRenderer(t, rich).Objects() {
		switch o := obj.(type) {
		case *canvas.Text:
			texts = append(texts, o.Text)
			ends = append(ends, o.Position().X+o.MinSize().Width)
		case *fyne.Container:
			link := o.Objects[0].(*Hyperlink)
			texts = append(texts, link.Text)
			ends = append(ends, o.Position().X+fyne.MeasureText(link.Text, theme.TextSize(), link.TextStyle).Width)
		}
	}
	return texts, ends
}

func TestRichText_TruncateMiddle_MultipleSegments(t *testing.T) {
	test.NewTempApp(t)

	dir := "/home/user/projects/"
	file := "fyne/widget/richtext.go"
	text := NewRichText(
		&TextSegment{Text: dir, Style: RichTextStyleInline},
		&TextSegment{Text: file, Style: RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}}},
	)
	text.Truncation = fyne.TextTruncateMiddle
	innerPad := theme.InnerPadding()

	text.Resize(fyne.NewSize(1000, 100))
	texts, _ := richTextRowTexts(t, text)
	assert.Equal(t, []string{dir, file}, texts, "a row that fits is not truncated")

	for _, width := range []float32{60, 100, 150, 200} {
		text.Resize(fyne.NewSize(width, 100))
		texts, ends := richTextRowTexts(t, text)
		assert.Equal(t, 2, len(texts))

		drawn := strings.Join(texts, "")
		if !assert.Equal(t, 1, strings.Count(drawn, "…"), drawn) {
			continue
		}
		parts := strings.Split(drawn, "…")
		assert.NotEmpty(t, parts[0], drawn)
		assert.True(t, strings.HasPrefix(dir+file, parts[0]), drawn)
		assert.True(t, strings.HasSuffix(dir+file, parts[1]), drawn)
		assert.True(t, strings.HasSuffix(drawn, "o"), drawn)
		assert.LessOrEqual(t, ends[1], width-innerPad, drawn)
	}
}

func TestRichText_TruncateMiddle_Hyperlink(t *testing.T) {
	test.NewTempApp(t)

	link := "example.com/some/very/long/path/index.html"
	text := NewRichText(&TextSegment{Text: "See ", Style: RichTextStyleInline}, &HyperlinkSegment{Text: link})
	text.Truncation = fyne.TextTruncateMiddle
	text.Resize(fyne.NewSize(150, 100))

	texts, ends := richTextRowTexts(t, text)
	assert.Equal(t, 2, len(texts))
	assert.Equal(t, "See ", texts[0])
	assert.Contains(t, texts[1], "…")
	assert.True(t, strings.HasPrefix(texts[1], "ex"), texts[1])
	assert.True(t, strings.HasSuffix(texts[1], ".html"), texts[1])
	assert.LessOrEqual(t, ends[1], 150-theme.InnerPadding())

	single := NewRichText(&HyperlinkSegment{Text: link})
	single.Truncation = fyne.TextTruncateMiddle
	single.Resize(fyne.NewSize(150, 100))
	texts, ends = richTextRowTexts(t, single)
	assert.Equal(t, 1, len(texts))
	assert.Contains(t, texts[0], "…")
	assert.True(t, strings.HasPrefix(texts[0], "ex"), texts[0])
	assert.True(t, strings.HasSuffix(texts[0], ".html"), texts[0])
	assert.LessOrEqual(t, ends[0], 150-theme.InnerPadding())
}

// TestRichText_TruncateWrappedHyperlink fills the height with a wrapped link, so
// the last row it keeps is cut short with an ellipsis. That row's segment is a
// HyperlinkSegment, not a TextSegment.
func TestRichText_TruncateWrappedHyperlink(t *testing.T) {
	test.NewTempApp(t)

	for _, trunc := range []fyne.TextTruncation{fyne.TextTruncateEllipsis, fyne.TextTruncateMiddle} {
		for _, wrap := range []fyne.TextWrap{fyne.TextWrapWord, fyne.TextWrapBreak} {
			t.Run(fmt.Sprint(trunc, wrap), func(t *testing.T) {
				text := NewRichText(&HyperlinkSegment{Text: "a long hyperlink text that wraps over many lines of the widget"})
				text.Wrapping = wrap
				text.Truncation = trunc
				assert.NotPanics(t, func() { text.Resize(fyne.NewSize(100, 40)) })

				texts, ends := richTextRowTexts(t, text)
				assert.NotEmpty(t, texts)
				assert.True(t, strings.HasSuffix(texts[len(texts)-1], "…"), texts[len(texts)-1])
				for _, end := range ends {
					assert.LessOrEqual(t, end, float32(100))
				}
			})
		}
	}
}

func TestRichText_TruncateMiddle_Quoted(t *testing.T) {
	test.NewTempApp(t)

	quote := RichTextStyle{Inline: true, QuotingDepth: 1}
	text := NewRichText(
		&TextSegment{Text: "/home/user/projects/", Style: quote},
		&TextSegment{Text: "fyne/widget/richtext.go", Style: quote},
	)
	text.Truncation = fyne.TextTruncateMiddle
	text.Resize(fyne.NewSize(150, 100))

	texts, ends := richTextRowTexts(t, text)
	assert.Equal(t, 2, len(texts))
	assert.Contains(t, strings.Join(texts, ""), "…")
	assert.LessOrEqual(t, ends[1], 150-theme.InnerPadding(), "the quote indent is left out of the space for the row")
}

func TestRichText_TruncateMiddle_CheckBox(t *testing.T) {
	test.NewTempApp(t)

	item := "a/very/long/task/list/item/text.txt"
	text := NewRichText(&CheckBoxSegment{Text: "x"}, &TextSegment{Text: item, Style: RichTextStyleInline})
	text.Truncation = fyne.TextTruncateMiddle
	text.Resize(fyne.NewSize(150, 100))

	objs := test.TempWidgetRenderer(t, text).Objects()
	assert.Equal(t, 2, len(objs))
	drawn := objs[1].(*canvas.Text)
	assert.Contains(t, drawn.Text, "…")
	assert.True(t, strings.HasPrefix(drawn.Text, "a/"), drawn.Text)
	assert.True(t, strings.HasSuffix(drawn.Text, ".txt"), drawn.Text)
	assert.LessOrEqual(t, drawn.Position().X+drawn.MinSize().Width, 150-theme.InnerPadding())
}

func TestText_findSpaceIndex(t *testing.T) {
	for name, tt := range map[string]struct {
		text string
		want int
	}{
		"no_space_fallback": {
			text: "iiiiiiiiiimmmmmmmmmm",
			want: -1,
		},
		"single_space": {
			text: "foobar foobar",
			want: 6,
		},
		"double_space": {
			text: "ww wwww www",
			want: 7,
		},
		"many_spaces": {
			text: "ww wwww www wwwww",
			want: 11,
		},
		"space beginning": {
			text: " ww",
			want: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, findSpaceIndex([]rune(tt.text), len(tt.text)-1))
		})
	}
}

// keptWrapSpaces counts the rows of rt that end in a space which does not fit in the
// width available, so it can only be there because word wrap broke the line on it.
func keptWrapSpaces(rt *RichText, textSize float32, style fyne.TextStyle) int {
	avail := rt.Size().Width - 2*theme.InnerPadding() + 2*rt.inset.Width
	kept := 0
	for i := 0; i < rt.rows(); i++ {
		row := rt.row(i)
		if len(row) > 1 && row[len(row)-1] == ' ' && fyne.MeasureText(string(row), textSize, style).Width > avail {
			kept++
		}
	}
	return kept
}

// Word wrap keeps the space that it broke a line on, when that space ends the line,
// so that the position after it is inside a row. It must not be drawn or measured
// though: a wrapped label has to size, align and truncate exactly as it did when the
// space was dropped, which is how the same label without the space still looks.
func TestLabel_WrapWord_KeptTrailingSpaceNotDrawn(t *testing.T) {
	test.NewTempApp(t)
	text, trimmed := "aa bb cc \nmore text", "aa bb cc\nmore text"
	newLabel := func(text string, width, height float32, align fyne.TextAlign, trunc fyne.TextTruncation) *Label {
		l := NewLabelWithStyle(text, align, fyne.TextStyle{})
		l.Wrapping = fyne.TextWrapWord
		l.Truncation = trunc
		test.TempWidgetRenderer(t, l)
		l.Resize(fyne.NewSize(width, height))
		return l
	}

	kept := 0
	for width := float32(20); width <= 160; width++ {
		l := newLabel(text, width, 500, fyne.TextAlignLeading, fyne.TextTruncateOff)
		if keptWrapSpaces(l.provider, theme.TextSize(), fyne.TextStyle{}) == 0 {
			continue
		}
		kept++

		for _, align := range []fyne.TextAlign{fyne.TextAlignLeading, fyne.TextAlignCenter, fyne.TextAlignTrailing} {
			for _, trunc := range []fyne.TextTruncation{fyne.TextTruncateOff, fyne.TextTruncateEllipsis, fyne.TextTruncateClip} {
				for _, height := range []float32{40, 60, 500} {
					with := newLabel(text, width, height, align, trunc)
					without := newLabel(trimmed, width, height, align, trunc)
					name := fmt.Sprintf("width %v align %v truncation %v height %v", width, align, trunc, height)

					assert.Equal(t, without.provider.rows(), with.provider.rows(), name)
					assert.Equal(t, without.MinSize(), with.MinSize(), name)
					got, want := richTextRenderTexts(with.provider), richTextRenderTexts(without.provider)
					if assert.Len(t, got, len(want), name) {
						for i := range want {
							assert.Equal(t, want[i].Text, got[i].Text, name)
							assert.Equal(t, want[i].Position(), got[i].Position(), name)
							assert.Equal(t, want[i].Size(), got[i].Size(), name)
						}
					}
				}
			}
		}
	}
	assert.NotZero(t, kept, "no width wrapped the line on its trailing space")
}

// A kept wrap space must not take room from the inline segment that follows it either,
// so text in several styles wraps exactly as it does when the space is not there.
func TestRichText_WrapWord_KeptTrailingSpaceInlineSegments(t *testing.T) {
	test.NewTempApp(t)
	bold := RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}}
	newRichText := func(width float32, segs ...RichTextSegment) *RichText {
		rt := NewRichText(segs...)
		rt.Wrapping = fyne.TextWrapWord
		test.TempWidgetRenderer(t, rt)
		rt.Resize(fyne.NewSize(width, 500))
		return rt
	}
	newStyled := func(first string, width float32) *RichText {
		return newRichText(
			width,
			&TextSegment{Style: bold, Text: first},
			&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Italic: true}}, Text: ". dd ee"},
			&HyperlinkSegment{Text: "link"},
		)
	}

	kept := 0
	for width := float32(20); width <= 120; width += .25 {
		alone := newRichText(width, &TextSegment{Style: bold, Text: "aa bb cc "})
		if keptWrapSpaces(alone, theme.TextSize(), bold.TextStyle) == 0 {
			continue
		}
		kept++

		with, without := newStyled("aa bb cc ", width), newStyled("aa bb cc", width)
		name := fmt.Sprintf("width %v", width)
		if assert.Len(t, with.rowBounds, len(without.rowBounds), name) {
			for i, want := range without.rowBounds {
				got := with.rowBounds[i]
				assert.Len(t, got.segments, len(want.segments), name)
				assert.Equal(t, want.begin, got.begin, name)
				assert.Equal(t, want.end, got.end, name)
			}
		}
		assert.Equal(t, without.MinSize(), with.MinSize(), name)
	}
	assert.NotZero(t, kept, "no width wrapped the first segment on its trailing space")
}

func TestRichText_WrapWord_EllipsisKeepsWrapSpaceHidden(t *testing.T) {
	test.NewTempApp(t)

	// The second inline segment starts on the row the first one ends, so its first row is
	// narrower than the rest. When word wrap keeps "aa "'s space there and the ellipsis is then
	// applied to that row, the kept space must stay hidden, as it does without the ellipsis.
	newInline := func(first string, align fyne.TextAlign, trunc fyne.TextTruncation, size fyne.Size) *RichText {
		style := RichTextStyleInline
		style.Alignment = align
		rt := NewRichText(
			&TextSegment{Style: style, Text: "xxxxxx"},
			&TextSegment{Style: style, Text: first + "\nbb cc dd"},
		)
		rt.Wrapping = fyne.TextWrapWord
		rt.Truncation = trunc
		test.TempWidgetRenderer(t, rt)
		rt.Resize(size)
		return rt
	}

	checked := 0
	for _, align := range []fyne.TextAlign{fyne.TextAlignLeading, fyne.TextAlignCenter, fyne.TextAlignTrailing} {
		for width := float32(40); width <= 120; width++ {
			for _, height := range []float32{40, 60, 80} {
				size := fyne.NewSize(width, height)
				// only sizes where word wrap breaks on the space; elsewhere "aa " fits and is drawn
				wrapped := false
				for _, b := range newInline("aa ", align, fyne.TextTruncateOff, size).rowBounds {
					wrapped = wrapped || b.wrapSpace
				}
				if !wrapped {
					continue
				}
				checked++

				with := newInline("aa ", align, fyne.TextTruncateEllipsis, size)
				without := newInline("aa", align, fyne.TextTruncateEllipsis, size)
				name := fmt.Sprintf("align %d size %vx%v", align, width, height)
				got, want := richTextRenderTexts(with), richTextRenderTexts(without)
				if !assert.Len(t, got, len(want), name) {
					continue
				}
				for i := range want {
					assert.Equal(t, want[i].Text, got[i].Text, name)
					assert.Equal(t, want[i].Position(), got[i].Position(), name)
					assert.Equal(t, want[i].Size(), got[i].Size(), name)
				}
			}
		}
	}
	assert.NotZero(t, checked, "no size wrapped the second segment on its trailing space")
}
