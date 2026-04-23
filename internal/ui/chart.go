package ui

import (
	"image"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const chartSamples = 180 // ~3 min a 1 Hz de bytecount.

// TrafficChart renderiza dos series (RX cyan, TX magenta) sobre un canvas
// raster. Se refresca llamando Push(rx, tx) cuando llega un bytecount.
type TrafficChart struct {
	widget.BaseWidget

	mu      sync.Mutex
	rxBuf   []float64
	txBuf   []float64
	raster  *canvas.Raster
	minSize fyne.Size
}

// NewTrafficChart construye el widget. minSize define el espacio mínimo
// que pide al layout (el widget crece si el contenedor lo permite).
func NewTrafficChart(minSize fyne.Size) *TrafficChart {
	c := &TrafficChart{
		rxBuf:   make([]float64, chartSamples),
		txBuf:   make([]float64, chartSamples),
		minSize: minSize,
	}
	c.ExtendBaseWidget(c)
	c.raster = canvas.NewRaster(c.draw)
	return c
}

// Push agrega una muestra (bytes/s) al buffer circular y dispara un
// redibujo del raster. Es seguro llamar desde cualquier goroutine.
func (c *TrafficChart) Push(rx, tx float64) {
	c.mu.Lock()
	c.rxBuf = append(c.rxBuf[1:], rx)
	c.txBuf = append(c.txBuf[1:], tx)
	c.mu.Unlock()
	canvas.Refresh(c.raster)
}

// Reset vuelve el chart a ceros (al desconectar, por ejemplo).
func (c *TrafficChart) Reset() {
	c.mu.Lock()
	for i := range c.rxBuf {
		c.rxBuf[i] = 0
		c.txBuf[i] = 0
	}
	c.mu.Unlock()
	canvas.Refresh(c.raster)
}

// CreateRenderer devuelve el renderer que empaqueta el raster dentro del
// bounding box del widget.
func (c *TrafficChart) CreateRenderer() fyne.WidgetRenderer {
	return &chartRenderer{chart: c}
}

type chartRenderer struct {
	chart *TrafficChart
}

func (r *chartRenderer) Layout(size fyne.Size) { r.chart.raster.Resize(size) }
func (r *chartRenderer) MinSize() fyne.Size    { return r.chart.minSize }
func (r *chartRenderer) Refresh()              { canvas.Refresh(r.chart.raster) }
func (r *chartRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.chart.raster}
}
func (r *chartRenderer) Destroy() {}

// draw produce el contenido del raster a partir del buffer actual.
// Diseño minimalista: grid suave, área rellena bajo cada serie con
// gradiente alpha, y línea anti-alias encima.
func (c *TrafficChart) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	// Fondo.
	fill(img, colorPanel)

	// Grid.
	for i := 1; i < 5; i++ {
		y := h * i / 5
		hline(img, 0, w, y, colorGrid)
	}
	for i := 1; i < 6; i++ {
		x := w * i / 6
		vline(img, x, 0, h, colorGrid)
	}

	c.mu.Lock()
	rx := append([]float64(nil), c.rxBuf...)
	tx := append([]float64(nil), c.txBuf...)
	c.mu.Unlock()

	rawMax := 1.0
	for i := range rx {
		if rx[i] > rawMax {
			rawMax = rx[i]
		}
		if tx[i] > rawMax {
			rawMax = tx[i]
		}
	}
	max := niceMax(rawMax * 1.15)

	drawSeries(img, w, h, rx, max, colorCyan, colorCyanDim)
	drawSeries(img, w, h, tx, max, colorMagenta, colorMagDim)

	return img
}

// drawSeries dibuja una serie: área bajo la curva (alpha decreciente) +
// línea principal sólida + "glow" simulado con una línea más gruesa y
// transparente detrás de la principal.
func drawSeries(img *image.NRGBA, w, h int, buf []float64, max float64, stroke, dim color.NRGBA) {
	if len(buf) < 2 {
		return
	}
	usableH := float64(h) * 0.92
	yFor := func(v float64) int {
		return h - int(v/max*usableH) - 1
	}
	xFor := func(i int) int {
		return int(float64(i) / float64(len(buf)-1) * float64(w-1))
	}

	// Área bajo la línea: para cada x de la curva, dibujar columna
	// vertical con alpha que decae hasta el borde inferior.
	for i := 0; i < len(buf)-1; i++ {
		x1, y1 := xFor(i), yFor(buf[i])
		x2, y2 := xFor(i+1), yFor(buf[i+1])
		if x1 == x2 {
			continue
		}
		for x := x1; x <= x2; x++ {
			t := float64(x-x1) / float64(x2-x1)
			y := int(float64(y1) + t*float64(y2-y1))
			for yy := y; yy < h; yy++ {
				// Alpha decae desde el tope de la curva hasta el borde.
				depth := 1.0 - float64(yy-y)/float64(h-y+1)
				if depth < 0 {
					depth = 0
				}
				alpha := uint8(float64(dim.A) * depth * 0.5)
				blendPixel(img, x, yy, color.NRGBA{dim.R, dim.G, dim.B, alpha})
			}
		}
	}

	// Glow: línea más gruesa y semi-transparente.
	for i := 0; i < len(buf)-1; i++ {
		thickLine(img, xFor(i), yFor(buf[i]), xFor(i+1), yFor(buf[i+1]), dim, 3)
	}
	// Línea principal.
	for i := 0; i < len(buf)-1; i++ {
		thickLine(img, xFor(i), yFor(buf[i]), xFor(i+1), yFor(buf[i+1]), stroke, 1)
	}
}

// --- Helpers de dibujo ------------------------------------------------------

func fill(img *image.NRGBA, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func hline(img *image.NRGBA, x1, x2, y int, c color.NRGBA) {
	for x := x1; x < x2; x++ {
		img.SetNRGBA(x, y, c)
	}
}
func vline(img *image.NRGBA, x, y1, y2 int, c color.NRGBA) {
	for y := y1; y < y2; y++ {
		img.SetNRGBA(x, y, c)
	}
}

// thickLine traza una línea de (x1,y1) a (x2,y2) con grosor thickness
// (1..N píxeles) usando Bresenham con expansión perpendicular simple.
func thickLine(img *image.NRGBA, x1, y1, x2, y2 int, c color.NRGBA, thickness int) {
	dx := abs(x2 - x1)
	dy := -abs(y2 - y1)
	sx := sign(x2 - x1)
	sy := sign(y2 - y1)
	err := dx + dy
	x, y := x1, y1
	b := img.Bounds()
	for {
		for t := -thickness / 2; t <= thickness/2; t++ {
			for s := -thickness / 2; s <= thickness/2; s++ {
				px, py := x+s, y+t
				if px >= b.Min.X && px < b.Max.X && py >= b.Min.Y && py < b.Max.Y {
					blendPixel(img, px, py, c)
				}
			}
		}
		if x == x2 && y == y2 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

// blendPixel compone color src sobre el píxel actual (alpha compositing).
func blendPixel(img *image.NRGBA, x, y int, src color.NRGBA) {
	if src.A == 0 {
		return
	}
	d := img.NRGBAAt(x, y)
	a := float64(src.A) / 255
	inv := 1 - a
	d.R = uint8(float64(src.R)*a + float64(d.R)*inv)
	d.G = uint8(float64(src.G)*a + float64(d.G)*inv)
	d.B = uint8(float64(src.B)*a + float64(d.B)*inv)
	d.A = 255
	img.SetNRGBA(x, y, d)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func niceMax(v float64) float64 {
	if v <= 0 {
		return 1024
	}
	pow := math.Pow(10, math.Floor(math.Log10(v)))
	n := v / pow
	nice := 10.0
	switch {
	case n <= 1:
		nice = 1
	case n <= 2:
		nice = 2
	case n <= 5:
		nice = 5
	}
	return nice * pow
}
