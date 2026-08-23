package main

import (
	"image"
	"testing"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

func TestFieldStrokeOnlyOneColour(t *testing.T) {
	for _, tc := range []struct {
		name   string
		colors tokens.ColorTokens
	}{
		{"light", tokens.DefaultLight},
		{"dark", tokens.DefaultDark},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newField(new(app.Window), 400, 300)
			f.SetColors(tc.colors)
			f.applyPending()

			faces := f.patch.Faces()
			if len(faces) == 0 {
				t.Fatal("patch has no faces")
			}
			want := seenFromNRGBA(quietStroke(tc.colors))
			for i := range faces {
				if faces[i].FillMaterial != nil {
					t.Errorf("face %d: fill set, want none", i)
				}
				if faces[i].StrokeMaterial == nil {
					t.Errorf("face %d: no stroke", i)
					continue
				}
				got := faces[i].StrokeMaterial.Color
				if !got.Equal(want) {
					t.Errorf("face %d: stroke %v, want FocusRing %v", i, got, want)
				}
			}
		})
	}
}

func TestFieldLayerConstructs(t *testing.T) {
	obs := FieldLayer(new(app.Window), rx.Of(theme.Default()))
	w, err := collectOne(obs)
	if err != nil {
		t.Fatalf("FieldLayer subscribe: %v", err)
	}
	if w == nil {
		t.Fatal("FieldLayer produced no widget")
	}
	dims := drawOnce(t, image.Pt(320, 240), w)
	if dims.Size.X == 0 || dims.Size.Y == 0 {
		t.Errorf("FieldLayer widget produced zero dimensions: %v", dims)
	}
}

func TestNewFieldUsesWindowSize(t *testing.T) {
	f := newField(new(app.Window), unit.Dp(200), unit.Dp(100))
	if f.coveredW <= 200 || f.coveredH <= 100 {
		t.Errorf("covered %gx%g, want overfill past 200×100", f.coveredW, f.coveredH)
	}
}
