package plotutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

const ImgRes = 1080

type vect2 struct {
	x float32
	y float32
}

type vect3 struct {
	x float32
	y float32
	z float32
}

var bgCol color.RGBA = color.RGBA{R: 0x18, G: 0x18, B: 0x1B, A: 0xFF}

func absInt(x int) int {

	if x < 0 {
		return x * -1
	}

	return x

}

func sign(x int) int {

	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	}

	return 0

}

func CreateBuildImage(buildImageData []byte) (*bytes.Buffer, error) {

	dataLen := len(buildImageData)

	if dataLen%4 != 0 {
		return nil, fmt.Errorf("in CreateBuildImage:\ninvalid build image data length")
	}

	data := make([]float32, dataLen/4)
	byteReader := bytes.NewReader(buildImageData)
	err := binary.Read(byteReader, binary.LittleEndian, &data)
	if err != nil {
		return nil, fmt.Errorf("in CreateBuildImage:\n%w", err)
	}

	if len(data) < 14 || (len(data) > 14 && (len(data)-14)%9 != 0) {
		return nil, fmt.Errorf("in CreateBuildImage:\ninvalid build image data")
	}

	img := image.NewRGBA(image.Rect(0, 0, ImgRes, ImgRes))

	//draw background based on depth
	for i := range ImgRes {
		for j := range ImgRes {
			img.SetRGBA(i, j, bgCol)
		}
	}

	var v1, v2, v3, v4 vect3
	vect3s := [4]*vect3{&v1, &v2, &v3, &v4}
	var u1, u2, u3, u4 vect2
	vect2s := [4]*vect2{&u1, &u2, &u3, &u4}

	// projection matrix
	m00, m10, m20, m30 := data[0], data[1], data[2], data[3]
	m01, m11, m21, m31 := data[4], data[5], data[6], data[7]
	m03, m13, m23, m33 := data[8], data[9], data[10], data[11]

	// 2d offset
	xOffset, yOffset := data[12], data[13]

	for i := 14; i < len(data); i += 9 {

		faceDir, d1, d2 := data[i+3], data[i+4], data[i+5]

		v1.x, v1.y, v1.z = data[i], data[i+1], data[i+2]
		v2.x, v2.y, v2.z = data[i], data[i+1], data[i+2]
		v3.x, v3.y, v3.z = data[i], data[i+1], data[i+2]
		v4.x, v4.y, v4.z = data[i], data[i+1], data[i+2]

		if faceDir == 0 {
			v2.y += d1
			v3.y += d1
			v3.z += d2
			v4.z += d2
		} else if faceDir == 1 {
			v2.x += d1
			v3.x += d1
			v3.z += d2
			v4.z += d2
		} else if faceDir == 2 {
			v2.x += d1
			v3.x += d1
			v3.y += d2
			v4.y += d2
		}

		var min, max *vect2

		// project vectors to 2d
		for j := range 4 {

			v := vect3s[j]
			u := vect2s[j]

			u.x = v.x*m00 + v.y*m10 + v.z*m20 + m30
			u.y = v.x*m01 + v.y*m11 + v.z*m21 + m31
			w := v.x*m03 + v.y*m13 + v.z*m23 + m33

			// normalize and transform point
			u.x = (u.x/w+1)/2 + xOffset
			u.y = 1 - ((u.y/w+1)/2 + yOffset)

			if j == 0 {

				min = &vect2{x: u.x, y: u.y}
				max = &vect2{x: u.x, y: u.y}

			} else {

				if u.x < min.x {
					min.x = u.x
				}
				if u.x > max.x {
					max.x = u.x
				}
				if u.y < min.y {
					min.y = u.y
				}
				if u.y > max.y {
					max.y = u.y
				}

			}

		}

		xMinInt := int(math.Floor(float64(min.x * ImgRes)))
		yMinInt := int(math.Floor(float64(min.y * ImgRes)))
		xMaxInt := int(math.Ceil(float64(max.x * ImgRes)))
		yMaxInt := int(math.Ceil(float64(max.y * ImgRes)))

		xDiff := xMaxInt - xMinInt + 1
		yDiff := yMaxInt - yMinInt + 1

		left := make([]int, yDiff)
		right := make([]int, yDiff)

		for j := range yDiff {
			left[j] = xDiff
			right[j] = -1
		}

		for j := range 4 {

			a := vect2s[j]
			b := vect2s[(j+1)%4]

			x := int(math.Round(float64(a.x*ImgRes))) - xMinInt
			y := int(math.Round(float64(a.y*ImgRes))) - yMinInt
			x1 := int(math.Round(float64(b.x*ImgRes))) - xMinInt
			y1 := int(math.Round(float64(b.y*ImgRes))) - yMinInt

			dx := absInt(x1 - x)
			dy := absInt(y1 - y)
			sx := sign(x1 - x)
			sy := sign(y1 - y)

			err := dx - dy

			for {

				if y >= 0 && y < yDiff {
					if x < left[y] {
						left[y] = x
					}
					if x > right[y] {
						right[y] = x
					}
				}

				if x == x1 && y == y1 {
					break
				}

				err2 := err * 2

				if err2 > -dy {
					err -= dy
					x += sx
				}

				if err2 < dx {
					err += dx
					y += sy
				}

			}

		}

		rc, gc, bc := uint8(data[i+6]), uint8(data[i+7]), uint8(data[i+8])

		// draw face
		for j := range yDiff {
			for k := left[j]; k <= right[j]; k++ {

				xPix := k + xMinInt
				yPix := j + yMinInt

				if xPix >= 0 && xPix < ImgRes && yPix >= 0 && yPix < ImgRes {
					img.SetRGBA(xPix, yPix, color.RGBA{R: rc, G: gc, B: bc, A: 0xFF})
				}

			}
		}

	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("in CreateBuildImage:\n%w", err)
	}

	return &buf, nil

}
