// Package place was absorbed from the retired github.com/vibrantgio/place repo.
package place

import "image"

// Place is used to place a rectangle of given size inside the passed in rect using
// the anchor point (ax,ay). Where both ax and ay are in the range [0..1] with (0,0)
// aligning the left,top corners of the rectangles and (1,1) aligning the right,bottom
// corners of the rectangles.
func Place(rect image.Rectangle, size image.Point, ax, ay float32) image.Rectangle {
	px, py := rect.Min.X+int(ax*float32(rect.Dx()-size.X)), rect.Min.Y+int(ay*float32(rect.Dy()-size.Y))
	return image.Rect(px, py, px+size.X, py+size.Y)
}
