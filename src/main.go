package main

import (
	"fmt"
	"path/filepath"
)

func main() {
	fmt.Print("Enter an .obj file path: ")
	var path string
	fmt.Scanln(&path)

	fmt.Print("Enter depth: ")
	maxDepth := 5
	_, _ = fmt.Scanln(&maxDepth)

	obj := readObjFile(path)
	fmt.Printf("Loaded %d vertices\n", len(obj.Vertices))
	fmt.Printf("Loaded %d faces\n", len(obj.Faces))

	octree := BuildOctreeFromObj(obj, maxDepth)
	outputPath := path[:len(path)-len(filepath.Ext(path))] + "_res.obj"
	stats, err := octree.WriteOccupiedVoxelsObj(outputPath)
	if err != nil {
		fmt.Printf("Failed to write result: %v\n", err)
		return
	}
	fmt.Printf("Voxels: %d\n", stats.Voxels)
	fmt.Printf("Vertices: %d\n", stats.Vertices)
	fmt.Printf("Faces: %d\n", stats.Faces)
	fmt.Printf("Result written to: %s\n", outputPath)
}
