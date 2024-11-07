package main

import (
	"distributed/stubs"
	"distributed/util"
)

func createWorld(width, extraHeight int) [][]uint8 {
	world := make([][]uint8, extraHeight)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

func GetImagePart(
	p stubs.Params,
	startY int,
	endY int,
	currentWorld [][]uint8) [][]uint8 {

	// calculate the dimensions of the piece of the image
	height := endY - startY
	width := p.ImageWidth

	// extra height for the first and last rows
	extraHeight := height + 2

	// create a slice to store the extended piece of the image
	nodeWorld := createWorld(width, extraHeight)

	// extract the piece of the image from the currentWorld
	for i := 0; i < extraHeight; i++ {
		for j := 0; j < width; j++ {
			// if including the first row then add the last row of the image on the top of the piece
			if i == 0 && startY == 0 {
				nodeWorld[i][j] = currentWorld[p.ImageHeight-1][j]

				// if including the last row then add the first row of the image on the bottom of the piece
			} else if i == height+1 && startY+height == p.ImageHeight {
				nodeWorld[i][j] = currentWorld[0][j]

				// otherwise just add the piece of the image
			} else {
				a := startY + i - 1
				nodeWorld[i][j] = currentWorld[a][j]
			}
		}
	}

	return nodeWorld
}

func CountAliveCells(world [][]uint8) int {
	var cells []util.Cell
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				cells = append(cells, util.Cell{X: j, Y: i})
			}
		}
	}
	return len(cells)
}
