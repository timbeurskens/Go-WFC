package WaveFunctionCollapse

import "image/color"

type WFCError string

func (e WFCError) Error() string {
	return string(e)
}

func addIfNotExists(color color.Color, colors *[]color.Color) int {
	for i, ref := range *colors {
		if ColorEquals(color, ref) {
			return i
		}
	}
	*colors = append(*colors, color)
	return len(*colors) - 1
}

type RGBA struct {
	R, G, B, A uint32
}

func NewRGBA(R, G, B, A uint32) RGBA {
	return RGBA{R, G, B, A}
}

func ColorEquals(c1, c2 color.Color) bool {
	rgba1 := NewRGBA(c1.RGBA())
	rgba2 := NewRGBA(c2.RGBA())

	return rgba1 == rgba2
}

func newUintMatrix(w, h int) [][]uint8 {
	mat := make([][]uint8, w)
	for i := range mat {
		mat[i] = make([]uint8, h)
	}
	return mat
}

func SymmetryFunc(symmetry string) (a, b func(int) int, cardinality int) {
	switch symmetry {
	case "L":
		cardinality = 4
		a = func(i int) int {
			return (i + 1) % 4
		}
		b = func(i int) int {
			if i%2 == 0 {
				return i + 1
			} else {
				return i - 1
			}
		}
	case "T":
		cardinality = 4
		a = func(i int) int {
			return (i + 1) % 4
		}
		b = func(i int) int {
			if i%2 == 0 {
				return i
			} else {
				return 4 - i
			}
		}
	case "I":
		cardinality = 2
		a = func(i int) int {
			return 1 - i
		}
		b = func(i int) int {
			return i
		}
	case "\\":
		cardinality = 2
		a = func(i int) int {
			return 1 - i
		}
		b = func(i int) int {
			return 1 - i
		}
	default:
		cardinality = 1
		a = func(i int) int {
			return i
		}
		b = func(i int) int {
			return i
		}
	}
	return
}

func RandomDistribution(a []float64, r float64) int {
	sum := SumDistribution(a)

	if sum == 0 {
		for j := range a {
			a[j] = 1
		}
		sum = SumDistribution(a)
	}

	for j := range a {
		a[j] /= sum
	}

	i := 0
	x := float64(0)
	n := len(a)

	for i < n {
		x += a[i]
		if r <= x {
			return i
		}
		i++
	}

	return 0
}

func SumDistribution(a []float64) (sum float64) {
	for _, v := range a {
		sum += v
	}
	return
}
