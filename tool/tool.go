package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io/ioutil"
	rand2 "math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"timbeurskens/WaveFunctionCollapse"
	"timbeurskens/progress"
	"time"
)

func init() {
	rand2.Seed(time.Now().UTC().UnixNano())
}

type Sample struct {
	Type        string `json:"type"`
	Name        string `json:"pattern,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	N           int    `json:"n,omitempty"`
	PeriodicIn  bool   `json:"periodic_in,omitempty"`
	PeriodicOut bool   `json:"periodic_out"`
	Symmetry    int    `json:"symmetry,omitempty"`
	Ground      int    `json:"ground,omitempty"`
	Black       bool   `json:"black,omitempty"`
	dir         string
}

var (
	file = flag.String("in", "samples.json", "json array of samples")
	reps = flag.Int("tries", 10, "The number of times to try and find a solution")
	limit = flag.Int("limit", 0, "Limit the number of iterations, 0 for infinity")
)

func main() {
	flag.Parse()
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}

	if _, err := os.Stat(*file); os.IsNotExist(err) {
		fmt.Println("File ", *file, " does not exist")
		return
	} else {
		dir = path.Dir(*file)
		fmt.Println("output directory: ", dir)
	}

	var sampleList []Sample

	if data, err := ioutil.ReadFile(*file); err != nil {
		fmt.Println(err)
		return
	} else if err = json.Unmarshal(data, &sampleList); err != nil {
		fmt.Println(err)
		return
	}

	var wg sync.WaitGroup
	n := len(sampleList)
	wg.Add(n)
	progress.Total(float64(n))
	progress.Start()
	for _, sample := range sampleList {
		sample.dir = dir
		go func(s Sample) {
			err := ExecuteSample(s)
			progress.Increment(1)

			//catch error to prevent breaking the goroutine
			if err != nil {
				fmt.Println(s.Name, err)
			}
			wg.Done()
		}(sample)
	}
	wg.Wait()
	progress.Stop()
	progress.Join()
}

func ExecuteSample(sample Sample) error {
	var model WaveFunctionCollapse.WFCModel
	var err error

	out := path.Join(sample.dir, OutputFile(sample.Name))

	switch sample.Type {
	case "overlapping":
		model, err = Overlapping(sample)
	case "tiled":
		model, err = Tiled(sample)
	default:
		model, err = nil, WaveFunctionCollapse.WFCError("type not recognized: "+sample.Type)
	}

	if err != nil {
		return err
	}

	err = ExecuteModel(model, out)

	return err
}

func ExecuteModel(model WaveFunctionCollapse.WFCModel, outfile string) error {
	result := false

	for k := 0; k < *reps; k++ {
		result = model.Run(*limit)
		if result {
			break
		}
	}

	if !result {
		return WaveFunctionCollapse.WFCError("contradiction")
	}

	if writer, err := os.Create(outfile); err != nil {
		return err
	} else if err = png.Encode(writer, model); err != nil {
		return err
	}

	return nil
}

func Tiled(sample Sample) (model WaveFunctionCollapse.WFCModel, err error) {
	info := WaveFunctionCollapse.ModelInfo{}

	if data, err := ioutil.ReadFile(path.Join(sample.dir, sample.Name)); err != nil {
		return nil, err
	} else if err = json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	for t := range info.Tiles {
		info.Tiles[t].Dir = path.Dir(path.Join(sample.dir, sample.Name))
	}

	info.Initialize()

	model = WaveFunctionCollapse.NewTiledModel(info, sample.Width, sample.Height, sample.PeriodicOut, sample.Black)
	return
}

func Overlapping(sample Sample) (model WaveFunctionCollapse.WFCModel, err error) {
	var img image.Image

	if patternfile, err := os.Open(path.Join(sample.dir, sample.Name)); err != nil {
		return nil, err
	} else if img, _, err = image.Decode(patternfile); err != nil {
		return nil, err
	}

	model = WaveFunctionCollapse.NewOverlappingModel(img, sample.N, sample.Width, sample.Height, sample.PeriodicIn,
		sample.PeriodicOut, sample.Symmetry, sample.Ground)
	return
}

func OutputFile(base string) (outfile string) {
	var randBytes [8]byte
	rand.Read(randBytes[:])
	outfile = strings.Replace(path.Base(base), path.Ext(base), "", 1)
	outfile += "_" + hex.EncodeToString(randBytes[:]) + ".png"
	return
}
