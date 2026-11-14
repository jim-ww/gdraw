package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// icon draws a glyph filling a size×size square.
type icon func(gtx C, size int, col color.NRGBA)

// materialIcon returns a Material Design icon, rotated by angle radians.
func materialIcon(data []byte, angle float32) icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err) // embedded icon data; cannot fail
	}
	return func(gtx C, size int, col color.NRGBA) {
		if angle != 0 {
			c := f32.Pt(float32(size)/2, float32(size)/2)
			defer op.Affine(f32.AffineId().Rotate(c, angle)).Push(gtx.Ops).Pop()
		}
		gtx.Constraints = layout.Exact(image.Pt(size, size))
		ic.Layout(gtx, col)
	}
}

// eraserIcon draws a tilted eraser: a solid tip and an outlined body.
func eraserIcon(gtx C, size int, col color.NRGBA) {
	s := float32(size) / 24
	pt := func(x, y float32) f32.Point { return f32.Pt(x*s, y*s) }
	rect := func(x0, y0, x1, y1 float32) clip.PathSpec {
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(pt(x0, y0))
		p.LineTo(pt(x1, y0))
		p.LineTo(pt(x1, y1))
		p.LineTo(pt(x0, y1))
		p.Close()
		return p.End()
	}
	rot := op.Affine(f32.AffineId().Rotate(pt(12, 12), -math.Pi/4)).Push(gtx.Ops)
	paint.FillShape(gtx.Ops, col, clip.Outline{Path: rect(3, 8, 10, 16)}.Op())
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: rect(3, 8, 21, 16), Width: 2 * s}.Op())
	rot.Pop()
}

var (
	toolIcons = [numTools]icon{
		ToolBrush:  materialIcon(icons.ImageBrush, 0),
		ToolLine:   materialIcon(icons.ContentRemove, -math.Pi/4),
		ToolArrow:  materialIcon(icons.NavigationArrowForward, -math.Pi/4),
		ToolRect:   materialIcon(icons.ImageCropSquare, 0),
		ToolCircle: materialIcon(icons.ImagePanoramaFishEye, 0),
		ToolText:   materialIcon(icons.EditorTextFields, 0),
		ToolFill:   materialIcon(icons.EditorFormatColorFill, 0),
		ToolEraser: eraserIcon,
		ToolSelect: materialIcon(icons.ContentSelectAll, 0),
		ToolMove:   materialIcon(icons.ActionOpenWith, 0),
		ToolPicker: materialIcon(icons.ImageColorize, 0),
	}

	// actionIcons are keyed by action name.
	actionIcons = map[string]icon{
		"open":        materialIcon(icons.FileFolderOpen, 0),
		"save":        materialIcon(icons.ContentSave, 0),
		"export":      materialIcon(icons.EditorInsertPhoto, 0),
		"undo":        materialIcon(icons.ContentUndo, 0),
		"redo":        materialIcon(icons.ContentRedo, 0),
		"size_dec":    materialIcon(icons.ContentRemove, 0),
		"size_inc":    materialIcon(icons.ContentAdd, 0),
		"zoom_out":    materialIcon(icons.ActionZoomOut, 0),
		"zoom_in":     materialIcon(icons.ActionZoomIn, 0),
		"toggle_fill": materialIcon(icons.ImageBrightness1, 0),
		"toggle_grid": materialIcon(icons.ImageGridOn, 0),
		"toggle_snap": materialIcon(icons.MapsMyLocation, 0),
		"new_tab":     materialIcon(icons.ContentAdd, 0),
		"help":        materialIcon(icons.ActionHelpOutline, 0),
	}

	closeIcon = materialIcon(icons.NavigationClose, 0)
)
