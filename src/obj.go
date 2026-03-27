package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

type Vector3f struct {
	X, Y, Z float64
}

func (a *Vector3f) Add(b Vector3f) Vector3f {
	return Vector3f{
		a.X + b.X,
		a.Y + b.Y,
		a.Z + b.Z,
	}
}

func (a *Vector3f) Sub(b Vector3f) Vector3f {
	return Vector3f{
		a.X - b.X,
		a.Y - b.Y,
		a.Z - b.Z,
	}
}

type Obj struct {
	Vertices []Vector3f
	Faces    [][3]int
}

func parseFaceIndex(token string) (int, error) {
	idxToken := token
	if slash := strings.IndexByte(token, '/'); slash != -1 {
		idxToken = token[:slash]
	}

	return strconv.Atoi(idxToken)
}

func readObjFile(path string) Obj {
	obj := Obj{
		Vertices: []Vector3f{},
		Faces:    [][3]int{},
	}

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		splitted := strings.Fields(line)

		if len(splitted) >= 4 {
			t := splitted[0]

			switch t {
			case "v":
				x, _ := strconv.ParseFloat(splitted[1], 64)
				y, _ := strconv.ParseFloat(splitted[2], 64)
				z, _ := strconv.ParseFloat(splitted[3], 64)
				obj.Vertices = append(obj.Vertices, Vector3f{x, y, z})
			case "f":
				v1, err1 := parseFaceIndex(splitted[1])
				v2, err2 := parseFaceIndex(splitted[2])
				v3, err3 := parseFaceIndex(splitted[3])
				if err1 != nil || err2 != nil || err3 != nil {
					continue
				}
				obj.Faces = append(obj.Faces, [3]int{v1, v2, v3})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return obj
}
