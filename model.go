package WaveFunctionCollapse

import (
	"image"
	"math"
	"math/rand"
)

type ModelResult uint8

const (
	ModelTrue ModelResult = iota
	ModelFalse
	ModelNull
)

var (
	Dx       = [4]int{-1, 0, 1, 0}
	Dy       = [4]int{0, 1, 0, -1}
	Opposite = [4]int{2, 3, 0, 1}
)

type IntTuple struct {
	A, B int
}

type WFCModel interface {
	image.Image
	Run(limit int) bool
}

type Model struct {
	Wave                                                 [][]bool
	Propagator                                           [4][][]int
	Compatible                                           [][][4]int
	Observed                                             []int
	Stack                                                []IntTuple
	StackSize                                            int
	Fmx, Fmy, T                                          int
	Periodic                                             bool
	Weights                                              []float64
	WeightLogWeights                                     []float64
	SumsOfOnes                                           []int
	SumOfWeights, SumOfWeightLogWeights, StartingEntropy float64
	SumsOfWeights, SumsOfWeightLogWeights, Entropies     []float64

	OnBoundary func(x, y int) bool `json:"-"`
	ImplClear  func()              `json:"-"`
}

func (model *Model) Run(limit int) bool {
	if model.Wave == nil {
		model.Init()
	}

	if model.ImplClear != nil {
		model.ImplClear()
	} else {
		model.ClearModel()
	}

	for l := 0; l < limit || limit == 0; l++ {
		result := model.Observe()
		if result != ModelNull {
			return result == ModelTrue
		}
		model.Propagate()
	}

	return true
}

func (model *Model) Init() {
	waveLength := model.Fmx * model.Fmy
	model.Wave = make([][]bool, waveLength)
	model.Compatible = make([][][4]int, waveLength)

	for i := range model.Wave {
		model.Wave[i] = make([]bool, model.T)
		model.Compatible[i] = make([][4]int, model.T)
	}

	model.WeightLogWeights = make([]float64, model.T)

	for t := range model.WeightLogWeights {
		model.WeightLogWeights[t] = model.Weights[t] * math.Log10(model.Weights[t])
		model.SumOfWeights += model.Weights[t]
		model.SumOfWeightLogWeights += model.WeightLogWeights[t]
	}

	model.StartingEntropy = math.Log10(model.SumOfWeights) - model.SumOfWeightLogWeights/model.SumOfWeights

	model.SumsOfOnes = make([]int, waveLength)
	model.SumsOfWeights = make([]float64, waveLength)
	model.SumsOfWeightLogWeights = make([]float64, waveLength)
	model.Entropies = make([]float64, waveLength)

	model.Stack = make([]IntTuple, waveLength*model.T)
}

func (model *Model) ClearModel() {
	numWeights := len(model.Weights)

	for i := range model.Wave {
		for t := 0; t < model.T; t++ {
			model.Wave[i][t] = true
			for d := 0; d < 4; d++ {
				model.Compatible[i][t][d] = len(model.Propagator[Opposite[d]][t])
			}
		}

		model.SumsOfOnes[i] = numWeights
		model.SumsOfWeights[i] = model.SumOfWeights
		model.SumsOfWeightLogWeights[i] = model.SumOfWeightLogWeights
		model.Entropies[i] = model.StartingEntropy
	}
}

func (model *Model) Observe() ModelResult {
	min := 1E+3
	argmin := -1

	for i := range model.Wave {
		if model.OnBoundary(i%model.Fmx, i/model.Fmx) {
			continue
		}

		amount := model.SumsOfOnes[i]

		if amount == 0 {
			return ModelFalse
		}

		entropy := model.Entropies[i]
		if amount <= 1 || !(entropy < min) {
			continue
		}
		noise := 1E-6 * rand.Float64()
		if !(entropy+noise < min) {
			continue
		}
		min = entropy + noise
		argmin = i
	}

	if argmin == -1 {
		model.Observed = make([]int, model.Fmx*model.Fmy)
		for i := range model.Wave {
			for t := 0; t < model.T; t++ {
				if model.Wave[i][t] {
					model.Observed[i] = t
					break
				}
			}
		}
		return ModelTrue
	}

	distribution := make([]float64, model.T)
	for t := range distribution {
		if model.Wave[argmin][t] {
			distribution[t] = model.Weights[t]
		} else {
			distribution[t] = 0
		}
	}

	r := RandomDistribution(distribution, rand.Float64())

	w := model.Wave[argmin]
	for t := 0; t < model.T; t++ {
		if w[t] != (t == r) {
			model.Ban(argmin, t)
		}
	}

	return ModelNull
}

func (model *Model) Propagate() {
	for model.StackSize > 0 {
		e1 := model.Stack[model.StackSize-1]
		model.StackSize--

		i1 := e1.A
		x1 := i1 % model.Fmx
		y1 := i1 / model.Fmx

		for d := 0; d < 4; d++ {
			dx, dy := Dx[d], Dy[d]
			x2, y2 := x1+dx, y1+dy
			if model.OnBoundary(x2, y2) {
				continue
			}

			if x2 < 0 {
				x2 += model.Fmx
			} else if x2 >= model.Fmx {
				x2 -= model.Fmx
			}

			if y2 < 0 {
				y2 += model.Fmy
			} else if y2 >= model.Fmy {
				y2 -= model.Fmy
			}

			i2 := x2 + y2*model.Fmx

			for _, t2 := range model.Propagator[d][e1.B] {
				model.Compatible[i2][t2][d]--
				if model.Compatible[i2][t2][d] == 0 {
					model.Ban(i2, t2)
				}
			}
		}
	}
}

func (model *Model) Ban(i, t int) {
	model.Wave[i][t] = false

	for d := 0; d < 4; d++ {
		model.Compatible[i][t][d] = 0
	}

	model.Stack[model.StackSize] = IntTuple{A: i, B: t}
	model.StackSize++

	sum := model.SumsOfWeights[i]
	model.Entropies[i] += model.SumsOfWeightLogWeights[i]/sum - math.Log10(sum)

	model.SumsOfOnes[i]--
	model.SumsOfWeights[i] -= model.Weights[t]
	model.SumsOfWeightLogWeights[i] -= model.WeightLogWeights[t]

	sum = model.SumsOfWeights[i]
	model.Entropies[i] -= model.SumsOfWeightLogWeights[i]/sum - math.Log10(sum)
}
