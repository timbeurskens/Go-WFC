package WaveFunctionCollapse

import (
	"image"
	"image/color"
	"math"
)

type OverlappingModel struct {
	*Model

	N        int
	Patterns [][]uint8
	Colors   []color.Color
	Ground   int
}

func (model *OverlappingModel) ColorModel() color.Model {
	return color.RGBAModel
}

func (model *OverlappingModel) Bounds() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{},
		Max: image.Point{
			X: model.Fmx,
			Y: model.Fmy,
		},
	}
}

func (model *OverlappingModel) At(x, y int) color.Color {
	if model.Model.Observed != nil {
		return model.ObservedColor(x, y)
	} else {
		return model.UnobservedColor(x, y)
	}
}

func (model *OverlappingModel) ObservedColor(x, y int) color.Color {
	var dx, dy int
	if y < model.Fmy-model.N+1 {
		dy = 0
	} else {
		dy = model.N - 1
	}
	if x < model.Fmx-model.N+1 {
		dx = 0
	} else {
		dx = model.N - 1
	}

	c := model.Colors[model.Patterns[model.Observed[x-dx+(y-dy)*model.Fmx]][dx+dy*model.N]]
	return model.ColorModel().Convert(c)
}

func (model *OverlappingModel) UnobservedColor(x, y int) color.Color {
	var contributors, r, g, b, a uint32
	for dy := 0; dy < model.N; dy++ {
		for dx := 0; dx < model.N; dx++ {
			sx, sy := x-dx, y-dy
			if sx < 0 {
				sx += model.Fmx
			}
			if sy < 0 {
				sy += model.Fmy
			}
			s := sx + sy*model.Fmx
			if model.OnBoundary(sx, sy) {
				continue
			}

			for t := 0; t < model.T; t++ {
				if model.Wave[s][t] {
					contributors++
					cr, cg, cb, ca := model.Colors[model.Patterns[t][dx+dy*model.N]].RGBA()

					r += cr
					g += cg
					b += cb
					a += ca
				}
			}
		}
	}

	if contributors == 0 {
		return model.ColorModel().Convert(color.RGBA{})
	}

	return model.ColorModel().Convert(color.RGBA{
		R: uint8(r / contributors),
		G: uint8(g / contributors),
		B: uint8(b / contributors),
		A: uint8(a / contributors),
	})
}

func (model *OverlappingModel) Clear() {
	model.Model.ClearModel()

	if model.Ground == 0 {
		return
	}

	for x := 0; x < model.Fmx; x++ {
		for t := 0; t < model.T; t++ {
			if t != model.Ground {
				model.Ban(x+(model.Fmy-1)*model.Fmx, t)
			}
		}
		for y := 0; y < model.Fmy-1; y++ {
			model.Ban(x+y*model.Fmx, model.Ground)
		}
	}

	model.Propagate()
}

func NewOverlappingModel(source image.Image, n, width, height int, periodicInput, periodicOutput bool, symmetry, ground int) (model *OverlappingModel) {
	//initialize model specific data
	model = &OverlappingModel{
		Model: &Model{
			Fmx:      width,
			Fmy:      height,
			Periodic: periodicOutput,
		},
		N:      n,
		Colors: make([]color.Color, 0),
	}

	//register abstract OnBoundary function
	model.Model.OnBoundary = model.OnBoundary

	//register virtual clear function
	model.Model.ImplClear = model.Clear

	smx, smy := source.Bounds().Dx(), source.Bounds().Dy()
	sample := newUintMatrix(smx, smy)

	weights := make(map[int64]int)
	ordering := make([]int64, 0)

	for x := 0; x < smx; x++ {
		for y := 0; y < smy; y++ {
			c := source.At(x, y)
			i := addIfNotExists(c, &model.Colors)
			sample[x][y] = uint8(i)
		}
	}

	PatternFromSample := func(x, y int) []uint8 {
		return model.Pattern(func(dx int, dy int) uint8 {
			return sample[(x+dx)%smx][(y+dy)%smy]
		})
	}

	Rotate := func(p []uint8) []uint8 {
		return model.Pattern(func(x int, y int) uint8 {
			return p[n-1-y+x*n]
		})
	}

	Reflect := func(p []uint8) []uint8 {
		return model.Pattern(func(x int, y int) uint8 {
			return p[n-1-x+y*n]
		})
	}

	var psw, psh int
	if periodicInput {
		psw, psh = smx, smy
	} else {
		psw, psh = smx-n+1, smy-n+1
	}

	//index patterns and calculate weights
	for y := 0; y < psh; y++ {
		for x := 0; x < psw; x++ {
			var ps [8][]uint8

			ps[0] = PatternFromSample(x, y) //original
			ps[1] = Reflect(ps[0])          //reflection
			ps[2] = Rotate(ps[0])           //rotation
			ps[3] = Reflect(ps[2])          //rotate-> reflect
			ps[4] = Rotate(ps[2])           //rotate -> rotate
			ps[5] = Reflect(ps[4])          //rotate -> rotate -> reflect
			ps[6] = Rotate(ps[4])           //rotate -> rotate -> rotate
			ps[7] = Reflect(ps[6])          //rotate -> rotate -> rotate -> reflect

			for k := 0; k < symmetry; k++ {
				index := model.Index(ps[k])
				if _, ok := weights[index]; !ok {
					weights[index]++
					ordering = append(ordering, index)
				}
				weights[index]++
			}
		}
	}

	model.T = len(weights)
	model.Ground = (ground + model.T) % model.T
	model.Patterns = make([][]uint8, model.T)
	model.Weights = make([]float64, model.T)

	for i, k := range ordering {
		model.Patterns[i] = model.PatternFromIndex(k)
		model.Weights[i] = float64(weights[k])
	}

	for d := range model.Propagator {
		model.Propagator[d] = make([][]int, model.T)
		for t := 0; t < model.T; t++ {
			list := make([]int, 0)
			for t2 := 0; t2 < model.T; t2++ {
				if model.Agrees(model.Patterns[t], model.Patterns[t2], Dx[d], Dy[d]) {
					list = append(list, t2)
				}
			}
			model.Propagator[d][t] = make([]int, len(list))
			copy(model.Propagator[d][t], list)
		}
	}

	return
}

func (model *OverlappingModel) Agrees(p1, p2 []uint8, dx, dy int) bool {
	var xmin, xmax, ymin, ymax int

	if dx < 0 {
		xmin, xmax = 0, dx+model.N
	} else {
		xmin, xmax = dx, model.N
	}

	if dy < 0 {
		ymin, ymax = 0, dy+model.N
	} else {
		ymin, ymax = dy, model.N
	}

	for y := ymin; y < ymax; y++ {
		for x := xmin; x < xmax; x++ {
			ip1, ip2 := x+model.N*y, x-dx+model.N*(y-dy)
			if p1[ip1] != p2[ip2] {
				return false
			}
		}
	}
	return true
}

func (model *OverlappingModel) PatternFromIndex(index int64) (result []uint8) {
	residue := index
	numColors := int64(len(model.Colors))
	power := int64(math.Pow(float64(numColors), float64(model.N*model.N)))
	result = make([]uint8, model.N*model.N)

	for i := range result {
		power /= numColors
		count := 0

		for residue >= power {
			residue -= power
			count++
		}

		result[i] = uint8(count)
	}
	return
}

func (model *OverlappingModel) Pattern(f func(int, int) uint8) []uint8 {
	result := make([]uint8, model.N*model.N)
	for y := 0; y < model.N; y++ {
		for x := 0; x < model.N; x++ {
			result[x+y*model.N] = f(x, y)
		}
	}
	return result
}

func (model *OverlappingModel) Index(p []uint8) int64 {
	result := int64(0)
	power := int64(1)
	patternSize := len(p)
	numColors := int64(len(model.Colors))
	for i := range p {
		result += int64(p[patternSize-1-i]) * power
		power *= numColors
	}
	return result
}

func (model *OverlappingModel) OnBoundary(x, y int) bool {
	return !model.Periodic && (x+model.N > model.Fmx || y+model.N > model.Fmy || x < 0 || y < 0)
}
