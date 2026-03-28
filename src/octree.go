package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sync"
)

const (
	axX = 0
	axY = 1
	axZ = 2
)

type OctreeNode struct {
	Position Vector3f
	Children [8]*OctreeNode
	IsLeaf   bool
	Occupied bool

	size     float64
	depth    int
	maxDepth int
	root     *OctreeNode

	created []int
	skipped []int

	mu        sync.Mutex
	splitOnce sync.Once
}

type ObjExportStats struct {
	Voxels   int
	Vertices int
	Faces    int
}

type voxelKey struct {
	X int
	Y int
	Z int
}

func NewOctreeNode(x, y, z float64, depth, maxDepth int, size float64, root *OctreeNode) *OctreeNode {
	return &OctreeNode{
		Position: Vector3f{X: x, Y: y, Z: z},
		IsLeaf:   true,
		Occupied: false,
		depth:    depth,
		maxDepth: maxDepth,
		size:     size,
		root:     root,
	}
}

func (n *OctreeNode) childCenter(index int) Vector3f {
	quarter := n.size / 4
	center := n.Position

	if (index & 1) == 1 {
		center.X += quarter
	} else {
		center.X -= quarter
	}

	if (index & 2) == 2 {
		center.Y += quarter
	} else {
		center.Y -= quarter
	}

	if (index & 4) == 4 {
		center.Z += quarter
	} else {
		center.Z -= quarter
	}

	return center
}

func (n *OctreeNode) split() {
	n.splitOnce.Do(func() {
		n.mu.Lock()
		defer n.mu.Unlock()

		if !n.IsLeaf {
			return
		}

		n.IsLeaf = false
		childSize := n.size / 2

		for i := range 8 {
			center := n.childCenter(i)
			n.Children[i] = NewOctreeNode(center.X, center.Y, center.Z, n.depth+1, n.maxDepth, childSize, n.root)
		}
	})
}

func (n *OctreeNode) InsertFace(face [3]Vector3f) {
	root := n.root
	if root == nil {
		root = n
	}

	n.mu.Lock()
	root.created[n.depth]++
	n.mu.Unlock()

	if !IsFaceOverlapWithVoxel(n.Position, n.size, face) {
		n.mu.Lock()
		root.skipped[n.depth]++
		n.mu.Unlock()
		return
	}

	if n.depth == n.maxDepth {
		n.mu.Lock()
		n.Occupied = true
		n.mu.Unlock()
		return
	}

	n.split()
	for _, child := range n.Children {
		child.InsertFace(face)
	}
}

func voxelKeyFromCenter(center, minCorner Vector3f, step float64) voxelKey {
	fx := math.Floor((center.X-minCorner.X)/step + 1e-9)
	fy := math.Floor((center.Y-minCorner.Y)/step + 1e-9)
	fz := math.Floor((center.Z-minCorner.Z)/step + 1e-9)

	return voxelKey{X: int(fx), Y: int(fy), Z: int(fz)}
}

func (n *OctreeNode) collectOccupiedVoxelKeys(occupied map[voxelKey]struct{}, minCorner Vector3f, step float64) {
	if n.IsLeaf {
		if n.Occupied {
			occupied[voxelKeyFromCenter(n.Position, minCorner, step)] = struct{}{}
		}
		return
	}

	for _, child := range n.Children {
		if child != nil {
			child.collectOccupiedVoxelKeys(occupied, minCorner, step)
		}
	}
}

func writeVoxelCubeObj(w *bufio.Writer, center Vector3f, size float64, vertexOffset int, stats *ObjExportStats, key voxelKey, occupied map[voxelKey]struct{}) (bool, error) {
	half := size / 2
	cx := center.X
	cy := center.Y
	cz := center.Z

	faceDefs := []struct {
		neighbor voxelKey
		triA     [3]int
		triB     [3]int
	}{
		{neighbor: voxelKey{X: 0, Y: 0, Z: -1}, triA: [3]int{1, 3, 2}, triB: [3]int{1, 4, 3}},
		{neighbor: voxelKey{X: 0, Y: 0, Z: 1}, triA: [3]int{5, 6, 7}, triB: [3]int{5, 7, 8}},
		{neighbor: voxelKey{X: 0, Y: -1, Z: 0}, triA: [3]int{1, 2, 6}, triB: [3]int{1, 6, 5}},
		{neighbor: voxelKey{X: 0, Y: 1, Z: 0}, triA: [3]int{4, 7, 3}, triB: [3]int{4, 8, 7}},
		{neighbor: voxelKey{X: -1, Y: 0, Z: 0}, triA: [3]int{1, 8, 4}, triB: [3]int{1, 5, 8}},
		{neighbor: voxelKey{X: 1, Y: 0, Z: 0}, triA: [3]int{2, 3, 7}, triB: [3]int{2, 7, 6}},
	}

	visibleFaces := make([][2][3]int, 0, 6)
	for _, def := range faceDefs {
		neighbor := voxelKey{X: key.X + def.neighbor.X, Y: key.Y + def.neighbor.Y, Z: key.Z + def.neighbor.Z}
		if _, exists := occupied[neighbor]; exists {
			continue
		}
		visibleFaces = append(visibleFaces, [2][3]int{def.triA, def.triB})
	}

	if len(visibleFaces) == 0 {
		return false, nil
	}

	verts := [8]Vector3f{
		{X: cx - half, Y: cy - half, Z: cz - half},
		{X: cx + half, Y: cy - half, Z: cz - half},
		{X: cx + half, Y: cy + half, Z: cz - half},
		{X: cx - half, Y: cy + half, Z: cz - half},
		{X: cx - half, Y: cy - half, Z: cz + half},
		{X: cx + half, Y: cy - half, Z: cz + half},
		{X: cx + half, Y: cy + half, Z: cz + half},
		{X: cx - half, Y: cy + half, Z: cz + half},
	}

	for _, v := range verts {
		if _, err := fmt.Fprintf(w, "v %.9f %.9f %.9f\n", v.X, v.Y, v.Z); err != nil {
			return false, err
		}
		stats.Vertices++
	}

	for _, face := range visibleFaces {
		for _, tri := range face {
			if _, err := fmt.Fprintf(w, "f %d %d %d\n", tri[0]+vertexOffset, tri[1]+vertexOffset, tri[2]+vertexOffset); err != nil {
				return false, err
			}
			stats.Faces++
		}
	}

	return true, nil
}

func (n *OctreeNode) writeOccupiedVoxelsObj(w *bufio.Writer, vertexOffset *int, stats *ObjExportStats, minCorner Vector3f, step float64, occupied map[voxelKey]struct{}) error {
	if n.IsLeaf {
		if n.Occupied {
			stats.Voxels++
			key := voxelKeyFromCenter(n.Position, minCorner, step)
			wrote, err := writeVoxelCubeObj(w, n.Position, n.size, *vertexOffset, stats, key, occupied)
			if err != nil {
				return err
			}
			if wrote {
				*vertexOffset += 8
			}
		}
		return nil
	}

	for _, child := range n.Children {
		if child != nil {
			if err := child.writeOccupiedVoxelsObj(w, vertexOffset, stats, minCorner, step, occupied); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *OctreeNode) WriteOccupiedVoxelsObj(path string) (ObjExportStats, error) {
	file, err := os.Create(path)
	if err != nil {
		return ObjExportStats{}, err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	minCorner := Vector3f{
		X: n.Position.X - n.size/2,
		Y: n.Position.Y - n.size/2,
		Z: n.Position.Z - n.size/2,
	}
	step := n.size / math.Pow(2, float64(n.maxDepth))

	occupied := map[voxelKey]struct{}{}
	n.collectOccupiedVoxelKeys(occupied, minCorner, step)

	vertexOffset := 0
	stats := ObjExportStats{}
	if err := n.writeOccupiedVoxelsObj(w, &vertexOffset, &stats, minCorner, step, occupied); err != nil {
		return ObjExportStats{}, err
	}

	if err := w.Flush(); err != nil {
		return ObjExportStats{}, err
	}

	return stats, nil
}

func BuildOctreeFromObj(obj Obj, maxDepth int) *OctreeNode {
	if len(obj.Vertices) == 0 {
		root := NewOctreeNode(0, 0, 0, 0, maxDepth, 1, nil)
		root.root = root
		root.created = make([]int, maxDepth+1)
		root.skipped = make([]int, maxDepth+1)
		return root
	}

	minV := obj.Vertices[0]
	maxV := obj.Vertices[0]

	for _, v := range obj.Vertices[1:] {
		if v.X < minV.X {
			minV.X = v.X
		}
		if v.Y < minV.Y {
			minV.Y = v.Y
		}
		if v.Z < minV.Z {
			minV.Z = v.Z
		}
		if v.X > maxV.X {
			maxV.X = v.X
		}
		if v.Y > maxV.Y {
			maxV.Y = v.Y
		}
		if v.Z > maxV.Z {
			maxV.Z = v.Z
		}
	}

	center := Vector3f{
		X: (minV.X + maxV.X) / 2,
		Y: (minV.Y + maxV.Y) / 2,
		Z: (minV.Z + maxV.Z) / 2,
	}

	extentX := maxV.X - minV.X
	extentY := maxV.Y - minV.Y
	extentZ := maxV.Z - minV.Z
	size := math.Max(extentX, math.Max(extentY, extentZ))
	if size == 0 {
		size = 1
	}

	root := NewOctreeNode(center.X, center.Y, center.Z, 0, maxDepth, size, nil)
	root.root = root
	root.created = make([]int, maxDepth+1)
	root.skipped = make([]int, maxDepth+1)

	validFaces := make([][3]Vector3f, 0, len(obj.Faces))
	for _, f := range obj.Faces {
		i0 := f[0] - 1
		i1 := f[1] - 1
		i2 := f[2] - 1

		if i0 < 0 || i1 < 0 || i2 < 0 {
			continue
		}
		if i0 >= len(obj.Vertices) || i1 >= len(obj.Vertices) || i2 >= len(obj.Vertices) {
			continue
		}

		face := [3]Vector3f{obj.Vertices[i0], obj.Vertices[i1], obj.Vertices[i2]}
		validFaces = append(validFaces, face)
	}

	workerCount := 8
	faceChan := make(chan [3]Vector3f, workerCount)
	var wg sync.WaitGroup

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for face := range faceChan {
				root.InsertFace(face)
			}
		}()
	}

	for _, face := range validFaces {
		faceChan <- face
	}
	close(faceChan)
	wg.Wait()

	return root
}

func axisIntervalOverlap(a, b, rad float64) bool {
	var min, max float64
	if a < b {
		min = a
		max = b
	} else {
		min = b
		max = a
	}
	return min <= rad && max >= -rad
}

func toArray3(v Vector3f) [3]float64 {
	return [3]float64{v.X, v.Y, v.Z}
}

func sub3(a, b [3]float64) [3]float64 {
	return [3]float64{a[axX] - b[axX], a[axY] - b[axY], a[axZ] - b[axZ]}
}

func axisX(a, b [3]float64) float64 {
	return a[axZ]*b[axY] - a[axY]*b[axZ]
}

func axisY(a, b [3]float64) float64 {
	return -a[axZ]*b[axX] + a[axX]*b[axZ]
}

func axisZ(a, b [3]float64) float64 {
	return a[axY]*b[axX] - a[axX]*b[axY]
}

// reference: https://fileadmin.cs.lth.se/cs/Personal/Tomas_Akenine-Moller/code/tribox_tam.pdf
func IsFaceOverlapWithVoxel(center Vector3f, size float64, face [3]Vector3f) bool {
	center3 := toArray3(center)
	v := [3][3]float64{}
	for i := range 3 {
		v[i] = sub3(toArray3(face[i]), center3)
	}

	e := [3][3]float64{}
	e[0] = sub3(v[1], v[0])
	e[1] = sub3(v[2], v[1])
	e[2] = sub3(v[0], v[2])

	halfSize := size / 2

	e0 := e[0]
	e1 := e[1]
	e2 := e[2]

	fex := math.Abs(e0[axX])
	fey := math.Abs(e0[axY])
	fez := math.Abs(e0[axZ])

	// Edge e0 axis tests: X01, Y02, Z12
	if !axisIntervalOverlap(axisX(e0, v[0]), axisX(e0, v[2]), fez*halfSize+fey*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisY(e0, v[0]), axisY(e0, v[2]), fez*halfSize+fex*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisZ(e0, v[1]), axisZ(e0, v[2]), fey*halfSize+fex*halfSize) {
		return false
	}

	fex = math.Abs(e1[axX])
	fey = math.Abs(e1[axY])
	fez = math.Abs(e1[axZ])

	// Edge e1 axis tests: X01, Y02, Z0
	if !axisIntervalOverlap(axisX(e1, v[0]), axisX(e1, v[2]), fez*halfSize+fey*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisY(e1, v[0]), axisY(e1, v[2]), fez*halfSize+fex*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisZ(e1, v[0]), axisZ(e1, v[1]), fey*halfSize+fex*halfSize) {
		return false
	}

	fex = math.Abs(e2[axX])
	fey = math.Abs(e2[axY])
	fez = math.Abs(e2[axZ])

	// Edge e2 axis tests: X2, Y1, Z12
	if !axisIntervalOverlap(axisX(e2, v[0]), axisX(e2, v[1]), fez*halfSize+fey*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisY(e2, v[0]), axisY(e2, v[1]), fez*halfSize+fex*halfSize) {
		return false
	}
	if !axisIntervalOverlap(axisZ(e2, v[1]), axisZ(e2, v[2]), fey*halfSize+fex*halfSize) {
		return false
	}

	for axis := range 3 {
		minVal := v[0][axis]
		maxVal := v[0][axis]
		for i := 1; i < 3; i++ {
			if v[i][axis] < minVal {
				minVal = v[i][axis]
			}
			if v[i][axis] > maxVal {
				maxVal = v[i][axis]
			}
		}

		if minVal > halfSize || maxVal < -halfSize {
			return false
		}
	}

	normal := [3]float64{
		e0[axY]*e1[axZ] - e0[axZ]*e1[axY],
		e0[axZ]*e1[axX] - e0[axX]*e1[axZ],
		e0[axX]*e1[axY] - e0[axY]*e1[axX],
	}
	radius := halfSize * (math.Abs(normal[axX]) + math.Abs(normal[axY]) + math.Abs(normal[axZ]))
	planeDistance := normal[axX]*v[0][axX] + normal[axY]*v[0][axY] + normal[axZ]*v[0][axZ]
	if math.Abs(planeDistance) > radius {
		return false
	}

	return true
}
