package WaveFunctionCollapse

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path"
	"strconv"
	"strings"
)

const Separator = " "

type TiledModel struct {
	*Model

	Black     bool
	TileSize  int
	Tiles     [][]color.Color
	TileNames []string
}

func (model *TiledModel) ColorModel() color.Model {
	return color.RGBAModel
}

func (model *TiledModel) Bounds() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{},
		Max: image.Point{
			X: model.Fmx * model.TileSize,
			Y: model.Fmy * model.TileSize,
		},
	}
}

func (model *TiledModel) At(x, y int) color.Color {
	if model.Model.Observed != nil {
		return model.ObservedColor(x, y)
	} else {
		return model.UnobservedColor(x, y)
	}
}

func (model *TiledModel) ObservedColor(x, y int) color.Color {
	tx, ty := x/model.TileSize, y/model.TileSize
	xt, yt := x%model.TileSize, y%model.TileSize

	tile := model.Tiles[model.Observed[tx+ty*model.Fmx]]
	c := tile[xt+yt*model.TileSize]

	return model.ColorModel().Convert(c)
}

func (model *TiledModel) UnobservedColor(x, y int) color.Color {
	tx, ty := x/model.TileSize, y/model.TileSize
	xt, yt := x%model.TileSize, y%model.TileSize

	a := model.Wave[tx+ty*model.Fmx]

	var amount = 0
	var sum float64 = 0

	for _, b := range a {
		if b {
			amount++
		}
	}

	for t := 0; t < model.T; t++ {
		if a[t] {
			sum += model.Weights[t]
		}
	}

	lambda := 1.0 / sum

	if model.Black && amount == model.T {
		return model.ColorModel().Convert(color.Black)
	} else {
		var r, g, b, a float64 = 0, 0, 0, 0
		for t := 0; t < model.T; t++ {
			if model.Wave[tx+ty*model.Fmx][t] {
				cr, cg, cb, ca := model.Tiles[t][xt+yt*model.TileSize].RGBA()
				r += float64(cr) * model.Weights[t] * lambda
				g += float64(cg) * model.Weights[t] * lambda
				b += float64(cb) * model.Weights[t] * lambda
				a += float64(ca) * model.Weights[t] * lambda
			}
		}

		return model.ColorModel().Convert(color.RGBA{
			R: uint8(r),
			G: uint8(g),
			B: uint8(b),
			A: uint8(a),
		})
	}
}

type ModelInfo struct {
	Tiles []Tile `json:"tiles"`
	Edges []Edge `json:"edges"`
	Size  int    `json:"size"`
}

func (info *ModelInfo) Initialize() error {
	for t := range info.Tiles {
		if err := info.Tiles[t].LoadFiles(); err != nil {
			return err
		}
	}
	return nil
}

type Tile struct {
	Name     string   `json:"name"`
	Symmetry string   `json:"symmetry"`
	Unique   bool     `json:"unique"`
	Weight   float64  `json:"weight"`
	Files    []string `json:"files"`
	images   []image.Image
	Dir      string `json:"-"`
}

func (tile *Tile) LoadFiles() error {
	tile.images = make([]image.Image, len(tile.Files))
	for i, file := range tile.Files {
		if imgFile, err := os.Open(path.Join(tile.Dir, file)); err != nil {
			return err
		} else if img, _, err := image.Decode(imgFile); err != nil {
			return err
		} else {
			tile.images[i] = img
		}
	}
	return nil
}

type Edge [2]string

func ParseEdge(edge Edge) (leftname string, leftcardinal int, rightname string, rightcardinal int) {
	l, r := strings.Split(edge[0], Separator), strings.Split(edge[1], Separator)
	leftname = l[0]
	rightname = r[0]
	leftcardinal, rightcardinal = 0, 0

	if len(l) > 1 {
		leftcardinal, _ = strconv.Atoi(l[1])
	}

	if len(r) > 1 {
		rightcardinal, _ = strconv.Atoi(r[1])
	}

	return
}

func NewTiledModel(info ModelInfo, width, height int, periodic, black bool) (model *TiledModel) {
	model = &TiledModel{
		Model: &Model{
			Fmx:      width,
			Fmy:      height,
			Periodic: periodic,
		},
		Black:    black,
		TileSize: info.Size,
	}

	//register abstract OnBoundary function
	model.Model.OnBoundary = model.OnBoundary

	model.Tiles = make([][]color.Color, 0)
	model.TileNames = make([]string, 0)

	model.Weights = make([]float64, 0)

	action := make([][8]int, 0)
	firstOccurrence := make(map[string]int)

	for _, tile := range info.Tiles {
		a, b, cardinality := SymmetryFunc(tile.Symmetry)
		model.T = len(action)
		firstOccurrence[tile.Name] = model.T
		symmetryMap := make([][8]int, cardinality)
		for t := 0; t < cardinality; t++ {
			symmetryMap[t] = [8]int{
				model.T + t,
				model.T + a(t),
				model.T + a(a(t)),
				model.T + a(a(a(t))),
				model.T + b(t),
				model.T + b(a(t)),
				model.T + b(a(a(t))),
				model.T + b(a(a(a(t)))),
			}

			action = append(action, symmetryMap[t])
		}

		if tile.Unique {
			for t := 0; t < cardinality; t++ {
				img := tile.images[t]
				model.Tiles = append(model.Tiles, model.Tile(img.At))
				model.TileNames = append(model.TileNames, fmt.Sprintf("%s %d", tile.Name, t))
			}
		} else {
			img := tile.images[0]
			model.Tiles = append(model.Tiles, model.Tile(img.At))
			model.TileNames = append(model.TileNames, fmt.Sprintf("%s %d", tile.Name, 0))

			for t := 1; t < cardinality; t++ {
				model.Tiles = append(model.Tiles, model.Rotate(model.Tiles[model.T+t-1]))
				model.TileNames = append(model.TileNames, fmt.Sprintf("%s %d", tile.Name, t))
			}
		}

		for t := 0; t < cardinality; t++ {
			model.Weights = append(model.Weights, tile.Weight)
		}
	}

	model.T = len(action)

	var tempPropagator [4][][]bool
	for d := range model.Propagator {
		tempPropagator[d] = make([][]bool, model.T)
		model.Propagator[d] = make([][]int, model.T)

		for t := 0; t < model.T; t++ {
			tempPropagator[d][t] = make([]bool, model.T)
		}
	}

	for _, edge := range info.Edges {
		leftName, leftCardinal, rightName, rightCardinal := ParseEdge(edge)

		l := action[firstOccurrence[leftName]][leftCardinal]
		d := action[l][1]
		r := action[firstOccurrence[rightName]][rightCardinal]
		u := action[r][1]

		tempPropagator[0][r][l] = true
		tempPropagator[0][action[r][6]][action[l][6]] = true
		tempPropagator[0][action[l][4]][action[r][4]] = true
		tempPropagator[0][action[l][2]][action[r][2]] = true

		tempPropagator[1][u][d] = true
		tempPropagator[1][action[d][6]][action[u][6]] = true
		tempPropagator[1][action[u][4]][action[d][4]] = true
		tempPropagator[1][action[d][2]][action[u][2]] = true
	}

	for t2 := 0; t2 < model.T; t2++ {
		for t1 := 0; t1 < model.T; t1++ {
			tempPropagator[2][t2][t1] = tempPropagator[0][t1][t2]
			tempPropagator[3][t2][t1] = tempPropagator[1][t1][t2]
		}
	}

	var sparsePropagator [4][][]int
	for d := range sparsePropagator {
		sparsePropagator[d] = make([][]int, model.T)
		for t := range sparsePropagator[d] {
			sparsePropagator[d][t] = make([]int, 0)
		}
	}

	for d := 0; d < 4; d++ {
		for t1 := 0; t1 < model.T; t1++ {
			for t2 := 0; t2 < model.T; t2++ {
				if tempPropagator[d][t1][t2] {
					sparsePropagator[d][t1] = append(sparsePropagator[d][t1], t2)
				}
			}

			spCount := len(sparsePropagator[d][t1])
			model.Propagator[d][t1] = make([]int, spCount)
			for st := 0; st < spCount; st++ {
				model.Propagator[d][t1][st] = sparsePropagator[d][t1][st]
			}
		}
	}

	return
}

func (model *TiledModel) OnBoundary(x, y int) bool {
	return !model.Periodic && (x < 0 || y < 0 || x >= model.Fmx || y >= model.Fmy)
}

func (model *TiledModel) Tile(f func(int, int) color.Color) (result []color.Color) {
	result = make([]color.Color, model.TileSize*model.TileSize)
	for y := 0; y < model.TileSize; y++ {
		for x := 0; x < model.TileSize; x++ {
			result[x+y*model.TileSize] = f(x, y)
		}
	}
	return
}

func (model *TiledModel) Rotate(array []color.Color) []color.Color {
	return model.Tile(func(x int, y int) color.Color {
		return array[model.TileSize-1-y+x*model.TileSize]
	})
}
