package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fogleman/meshview"
)

func main() {
	fmt.Print("Enter an .obj file path (without folder): ")
	var path string
	fmt.Scanln(&path)
	fullPath := "../data/" + path
	fmt.Printf("Reading %s\n", fullPath)

	fmt.Print("Enter depth: ")
	maxDepth := 5
	_, _ = fmt.Scanln(&maxDepth)
	now := time.Now()

	obj := readObjFile(fullPath)
	fmt.Printf("Loaded Vertices: %d\n", len(obj.Vertices))
	fmt.Printf("Loaded Faces: %d\n", len(obj.Faces))

	octree := BuildOctreeFromObj(obj, maxDepth)
	fileName := filepath.Base(fullPath)
	outputPath := "../test/" + fileName[:len(fileName)-len(filepath.Ext(fileName))] + "_res.obj"
	stats, err := octree.WriteOccupiedVoxelsObj(outputPath)
	if err != nil {
		fmt.Printf("Failed to write result: %v\n", err)
		return
	}

	fmt.Printf("Voxels: %d\n", stats.Voxels)
	fmt.Printf("Vertices: %d\n", stats.Vertices)
	fmt.Printf("Faces: %d\n", stats.Faces)
	fmt.Println("Node Created:")
	for depth := 0; depth <= maxDepth; depth++ {
		fmt.Printf("%d : %d\n", depth, octree.created[depth])
	}
	fmt.Println("Node Skipped:")
	for depth := 0; depth <= maxDepth; depth++ {
		fmt.Printf("%d : %d\n", depth, octree.skipped[depth])
	}
	fmt.Printf("Depth: %d\n", maxDepth)

	fmt.Printf("Result written to: %s\n", outputPath)
	fmt.Printf("Time: %v\n", time.Since(now))
	meshview.Run(outputPath)
}
