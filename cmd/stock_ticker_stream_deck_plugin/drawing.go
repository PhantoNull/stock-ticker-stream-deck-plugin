package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var (
	white  = &color.RGBA{255, 255, 255, 255}
	orange = &color.RGBA{248, 136, 28, 255}
	green  = &color.RGBA{62, 158, 62, 255}
	red    = &color.RGBA{181, 26, 40, 255}
	blue   = &color.RGBA{61, 117, 164, 255}
	grey   = &color.RGBA{255, 255, 255, 255}
)

// DrawTile renders the tile given context and stock data
func DrawTile(title string, price, change, changePercent float32, status string, statusColor *color.RGBA, arrow string, arrowColor *color.RGBA) *[]byte {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(width)))
	backgroundAccent := &color.RGBA{40, 7, 10, 255}
	if changePercent >= 0 {
		backgroundAccent = &color.RGBA{14, 32, 19, 255}
	}
	drawVerticalGradient(0, 0, int(width), int(width), &color.RGBA{0, 0, 0, 255}, backgroundAccent, img)
	priceText := fmt.Sprintf("%.2f", price)
	priceFontSize := fitPriceFontSize(priceText, "Lato-Bold.ttf", 14, 42)
	drawLabel(&Label{
		text:     title,
		fontName: "Muli-ExtraBold.ttf",
		fontSize: 17,
		x:        4,
		y:        25,
		clr:      white,
	}, img)
	drawStatusIndicator(60, 15, status, statusColor, img)
	drawLine(5, 30, 11, 2, &color.RGBA{102, 102, 102, 255}, img)
	if arrow != "" {
		drawTriangleIndicator(60, 45, arrow, arrowColor, img)
	}
	drawLabel(&Label{
		text:     priceText,
		fontName: "Lato-Bold.ttf",
		fontSize: priceFontSize,
		x:        4,
		y:        50,
		clr:      white,
	}, img)
	changeColor := red
	if changePercent >= 0 {
		changeColor = green
	}
	drawLabel(&Label{
		text:     fmt.Sprintf("%.2f%%", changePercent),
		fontName: "Muli-ExtraBold.ttf",
		fontSize: 11,
		x:        4,
		y:        65,
		clr:      changeColor,
	}, img)
	b, err := EncodePNG(img)
	if err != nil {
		log.Fatalf("EncodePNG: %v\n", err)
	}
	return &b
}

// BuildHistoryTileSVG renders the historical chart as an SVG closely matching the Spotlight Coin visual style.
func BuildHistoryTileSVG(title string, price float32, points []float32, changePercent float32, rangeLabel string) string {
	points = aggregateSeries(points, 10)
	stroke := "#FF3B30"
	fillAccent := "#650212"
	arrow := "▼"
	if changePercent >= 0 {
		stroke = "#34C759"
		fillAccent = "#275C35"
		arrow = "▲"
	} else if changePercent == 0 {
		arrow = "■"
	}

	plotBottom := 86.0
	plotHeight := 22.0
	minVal, maxVal := points[0], points[0]
	for _, point := range points[1:] {
		if point < minVal {
			minVal = point
		}
		if point > maxVal {
			maxVal = point
		}
	}
	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1
	}

	stepX := 72.0 / float64(max(len(points)-1, 1))
	linePoints := make([]string, 0, len(points))
	areaPoints := make([]string, 0, len(points)+2)
	avg := float32(0)
	for i, point := range points {
		avg += point
		x := stepX * float64(i)
		y := plotBottom - (float64(point-minVal)/float64(valueRange))*plotHeight
		linePoints = append(linePoints, fmt.Sprintf("%.2f,%.2f", x, y))
		areaPoints = append(areaPoints, fmt.Sprintf("%.2f,%.2f", x, y))
	}
	avg /= float32(len(points))
	baselineY := plotBottom - (float64(avg-minVal)/float64(valueRange))*plotHeight
	areaPoints = append(areaPoints, "72,100", "0,100")

	return fmt.Sprintf(
		`<svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
<defs>
<linearGradient id="grad" x1="0" y1="0" x2="0" y2="1">
<stop offset="0%%" stop-color="#000000"/>
<stop offset="30%%" stop-color="#000000"/>
<stop offset="100%%" stop-color="%s"/>
</linearGradient>
<linearGradient id="areaGradient" x1="0" y1="0" x2="0" y2="1">
<stop offset="0%%" stop-color="%s"/>
<stop offset="100%%" stop-color="#000000"/>
</linearGradient>
</defs>
<rect width="100" height="100" fill="url(#grad)"/>
<line x1="0" y1="58" x2="0" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="25" y1="58" x2="25" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="50" y1="58" x2="50" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="75" y1="58" x2="75" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="100" y1="58" x2="100" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<polygon points="%s" fill="url(#areaGradient)" />
<polyline points="%s" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
<line x1="0" y1="%.2f" x2="100" y2="%.2f" stroke="%s" stroke-width="1.5" stroke-dasharray="6,2" opacity="0.55" />
<text x="6" y="20" font-size="17" font-weight="900" fill="white" font-family="Arial">%s</text>
<text x="84" y="20" font-size="12" font-weight="900" fill="white" font-family="Arial">%s</text>
<text x="6" y="42" font-size="17" font-weight="700" fill="white" font-family="Arial">%s</text>
<rect x="3" y="73" width="46" height="18" rx="4" ry="4" fill="#000000" fill-opacity="0.42"/>
<text x="6" y="88" font-size="17" font-weight="700" fill="%s" font-family="Arial">%s</text>
<text x="84" y="88" font-size="17" font-weight="900" fill="%s" font-family="Arial">%s</text>
</svg>`,
		fillAccent,
		fillAccent,
		strings.Join(areaPoints, " "),
		strings.Join(linePoints, " "),
		stroke,
		baselineY,
		baselineY,
		stroke,
		escapeSVGText(title),
		escapeSVGText(rangeLabel),
		escapeSVGText(formatHistoryPrice(price)),
		stroke,
		escapeSVGText(fmt.Sprintf("%s%%", formatCompactPercent(absf(changePercent)))),
		stroke,
		escapeSVGText(arrow),
	)
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func formatHistoryPrice(price float32) string {
	if price >= 1000 {
		return fmt.Sprintf("%.0f", price)
	}
	if price >= 100 {
		return fmt.Sprintf("%.2f", price)
	}
	return fmt.Sprintf("%.2f", price)
}

func formatCompactPercent(v float32) string {
	if math.Abs(float64(v)) >= 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func escapeSVGText(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(s)
}

func aggregateSeries(points []float32, target int) []float32 {
	if len(points) <= target || target < 2 {
		return points
	}
	if target == 2 {
		return []float32{points[0], points[len(points)-1]}
	}

	innerPoints := points[1 : len(points)-1]
	innerTarget := target - 2
	result := make([]float32, 0, target)
	result = append(result, points[0])

	if innerTarget > 0 && len(innerPoints) > 0 {
		bucketSize := float64(len(innerPoints)) / float64(innerTarget)
		for i := 0; i < innerTarget; i++ {
			start := int(float64(i) * bucketSize)
			end := int(float64(i+1) * bucketSize)
			if i == innerTarget-1 {
				end = len(innerPoints)
			}
			if end <= start {
				end = start + 1
			}
			if end > len(innerPoints) {
				end = len(innerPoints)
			}
			var sum float32
			for _, point := range innerPoints[start:end] {
				sum += point
			}
			result = append(result, sum/float32(end-start))
		}
	}

	result = append(result, points[len(points)-1])
	return result
}

const width = 72

type fpair struct {
	fontName string
	fontSize float64
}

// Label struct contains text, position and color information
type Label struct {
	text     string
	y        uint
	x        uint
	fontName string
	fontSize float64
	center   bool
	clr      *color.RGBA
}
type singleshared struct {
	fonts  map[string]*truetype.Font
	faces  map[fpair]font.Face
	pngEnc *png.Encoder
	pngBuf *bytes.Buffer
}

var sharedinstance *singleshared
var once sync.Once

func shared() *singleshared {
	once.Do(func() {
		sharedinstance = &singleshared{
			pngEnc: &png.Encoder{
				CompressionLevel: png.NoCompression,
			},
			pngBuf: bytes.NewBuffer(make([]byte, 0, 15697)),
		}
		sharedinstance.fonts = make(map[string]*truetype.Font)
		sharedinstance.faces = make(map[fpair]font.Face)
	})
	return sharedinstance
}

func (ss *singleshared) face(fontName string, fontSize float64) font.Face {
	if face, ok := ss.faces[fpair{fontName, fontSize}]; ok {
		return face
	}

	font := ss.fonts[fontName]
	if font == nil {
		b, err := ioutil.ReadFile(fontName)
		if err != nil {
			log.Fatal(err)
		}
		font, err = truetype.Parse(b)
		if err != nil {
			log.Fatal("failed to parse font")
		}
		ss.fonts[fontName] = font
	}

	face := truetype.NewFace(font, &truetype.Options{Size: fontSize, DPI: 72})
	ss.faces[fpair{fontName, fontSize}] = face

	return face
}

func unfix(x fixed.Int26_6) float64 {
	const shift, mask = 6, 1<<6 - 1
	if x >= 0 {
		return float64(x>>shift) + float64(x&mask)/64
	}
	x = -x
	if x >= 0 {
		return -(float64(x>>shift) + float64(x&mask)/64)
	}
	return 0
}

func drawLine(x, y, width, height int, c *color.RGBA, img *image.RGBA) {
	maxX := x + width - 1
	maxY := y + height - 1
	for x := x; x < maxX; x++ {
		for y := y; y < maxY; y++ {
			img.Set(x, y, c)
		}
	}
}

func drawTriangleIndicator(x, y int, direction string, c *color.RGBA, img *image.RGBA) {
	size := 8
	switch direction {
	case "^":
		for row := 0; row < size; row++ {
			startX := x - row
			endX := x + row
			for px := startX; px <= endX; px++ {
				drawPixel(px, y+row, c, img)
			}
		}
	case "v":
		for row := 0; row < size; row++ {
			startX := x - (size - 1 - row)
			endX := x + (size - 1 - row)
			for px := startX; px <= endX; px++ {
				drawPixel(px, y+row, c, img)
			}
		}
	}
}

func drawStatusIndicator(x, y int, status string, c *color.RGBA, img *image.RGBA) {
	switch status {
	case "sun":
		drawSunIndicator(x, y, c, img)
	case "pre":
		drawPreMarketIndicator(x, y, c, img)
	case "moon":
		drawMoonIndicator(x, y, c, img)
	}
}

func drawSunIndicator(cx, cy int, c *color.RGBA, img *image.RGBA) {
	for dx := -3; dx <= 3; dx++ {
		for dy := -3; dy <= 3; dy++ {
			if dx*dx+dy*dy <= 8 {
				drawPixel(cx+dx, cy+dy, c, img)
			}
		}
	}
	for _, ray := range [][4]int{
		{0, -6, 0, -4},
		{0, 4, 0, 6},
		{-6, 0, -4, 0},
		{4, 0, 6, 0},
		{-5, -5, -4, -4},
		{4, -5, 5, -4},
		{-5, 4, -4, 5},
		{4, 4, 5, 5},
	} {
		drawSegment(cx+ray[0], cy+ray[1], cx+ray[2], cy+ray[3], c, img)
	}
}

func drawPreMarketIndicator(cx, cy int, c *color.RGBA, img *image.RGBA) {
	for dx := -5; dx <= 5; dx++ {
		for dy := -5; dy <= 5; dy++ {
			distance := dx*dx + dy*dy
			if distance >= 18 && distance <= 28 {
				drawPixel(cx+dx, cy+dy, c, img)
			}
		}
	}
	for dx := 0; dx <= 5; dx++ {
		drawPixel(cx+dx, cy, c, img)
	}
	for dy := -5; dy <= 0; dy++ {
		drawPixel(cx, cy+dy, c, img)
	}
}

func drawMoonIndicator(cx, cy int, c *color.RGBA, img *image.RGBA) {
	for dx := -4; dx <= 4; dx++ {
		for dy := -5; dy <= 5; dy++ {
			if dx*dx+dy*dy <= 22 && (dx+2)*(dx+2)+dy*dy >= 18 {
				drawPixel(cx+dx, cy+dy, c, img)
			}
		}
	}
}

func drawSegment(x0, y0, x1, y1 int, c color.Color, img *image.RGBA) {
	dx := x1 - x0
	dy := y1 - y0
	steps := max(abs(dx), abs(dy))
	if steps == 0 {
		drawPixel(x0, y0, c, img)
		return
	}
	for i := 0; i <= steps; i++ {
		x := x0 + dx*i/steps
		y := y0 + dy*i/steps
		drawPixel(x, y, c, img)
	}
}

func drawPixel(x, y int, c color.Color, img *image.RGBA) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return
	}
	img.Set(x, y, c)
}

func drawThickPixel(x, y, thickness int, c color.Color, img *image.RGBA) {
	if thickness < 1 {
		thickness = 1
	}
	radius := thickness / 2
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			drawPixel(x+dx, y+dy, c, img)
		}
	}
}

func drawVerticalLine(x, y0, y1 int, c color.Color, img *image.RGBA) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		drawPixel(x, y, c, img)
	}
}

func drawVerticalGradient(x, y, width, height int, topColor, bottomColor *color.RGBA, img *image.RGBA) {
	if height <= 1 {
		return
	}
	for row := 0; row < height; row++ {
		progress := float64(row) / float64(height-1)
		clr := color.RGBA{
			R: uint8(float64(topColor.R)*(1-progress) + float64(bottomColor.R)*progress),
			G: uint8(float64(topColor.G)*(1-progress) + float64(bottomColor.G)*progress),
			B: uint8(float64(topColor.B)*(1-progress) + float64(bottomColor.B)*progress),
			A: uint8(float64(topColor.A)*(1-progress) + float64(bottomColor.A)*progress),
		}
		for col := 0; col < width; col++ {
			drawPixel(x+col, y+row, clr, img)
		}
	}
}

func drawGuideLines(x, y, width, height int, img *image.RGBA) {
	guideColor := &color.RGBA{255, 255, 255, 35}
	for step := 0; step < 4; step++ {
		col := x + step*(width-1)/3
		drawVerticalLine(col, y, height-1, guideColor, img)
	}
}

func fitPriceFontSize(text, fontName string, defaultSize float64, maxWidth float64) float64 {
	size := defaultSize
	for size > 8 && measureTextWidth(text, fontName, size) > maxWidth {
		size -= 0.5
	}
	return size
}

func measureTextWidth(text, fontName string, fontSize float64) float64 {
	face := shared().face(fontName, fontSize)
	var width float64
	for _, r := range text {
		advance, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}
		width += unfix(advance)
	}
	return width
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func drawLabel(l *Label, img *image.RGBA) {
	shared := shared()
	lines := strings.Split(l.text, "\n")
	curY := l.y
	face := shared.face(l.fontName, l.fontSize)

	for _, line := range lines {
		var lwidth float64
		for _, x := range line {
			awidth, ok := face.GlyphAdvance(rune(x))
			if ok != true {
				log.Println("drawLabel: Failed to GlyphAdvance")
				return
			}
			lwidth += unfix(awidth)
		}

		lx := float64(l.x)
		if l.center {
			lx = (float64(width) / 2.) - (lwidth / 2.)
		}
		point := fixed.Point26_6{X: fixed.Int26_6(lx * 64), Y: fixed.Int26_6(curY * 64)}

		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(l.clr),
			Face: face,
			Dot:  point,
		}
		d.DrawString(line)
		curY += 12
	}
}

// EncodePNG renders the current state of the graph
func EncodePNG(img *image.RGBA) ([]byte, error) {
	bak := append(img.Pix[:0:0], img.Pix...)
	shared := shared()
	err := shared.pngEnc.Encode(shared.pngBuf, img)
	if err != nil {
		return nil, err
	}
	img.Pix = bak
	bts := shared.pngBuf.Bytes()
	shared.pngBuf.Reset()
	return bts, nil
}
