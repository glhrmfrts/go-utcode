package utcode

import (
	"log"
	"math"
	"testing"
)

type Product struct {
	Name        string
	Description string
	Quantity    int
	Image       *ProductImage
}

type ProductImage struct {
	Large  string
	Medium string
	Small  string
}

func TestBoolEncode(t *testing.T) {
	val := true
	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := false
	Decode(data, &res)

	log.Printf("bool:\t%v -> %s -> %v", val, string(data), res)
}

func TestIntEncode(t *testing.T) {
	val := 616
	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := 0
	Decode(data, &res)

	log.Printf("int:\t%v -> %s -> %v", val, string(data), res)
}

func TestFloatEncode(t *testing.T) {
	val := math.Pi
	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := 0.0
	Decode(data, &res)

	log.Printf("float:\t%v -> %s -> %v", val, string(data), res)
}

func TestStringEncode(t *testing.T) {
	val := "The quick brown foxy"
	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := ""
	Decode(data, &res)

	log.Printf("string:\t%v -> %s -> %v", val, string(data), res)
}

func TestMapEncode(t *testing.T) {
	val := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := map[string]interface{}{}
	Decode(data, res)

	log.Printf("map:\t%v -> %s -> %v", val, string(data), res)
}

func TestSliceEncode(t *testing.T) {
	val := []string{
		"foo", "bar", "john", "doe",
	}

	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := []string{}
	Decode(data, &res)

	log.Printf("slice:\t%v -> %s -> %s", val, string(data), res)
}

func TestStructEncode(t *testing.T) {
	val := Product{
		Name: "Shirt",
		Description: "black shirt",
		Quantity: 5,
		Image: &ProductImage{
			Large: "large",
			Medium: "__medium",
			Small: "smallllll",
		},
	}

	data, err := Encode(val)
	if err != nil {
		t.Fatal(err)
	}

	res := &Product{}
	Decode(data, res)

	log.Printf("struct:\t%v -> %s -> %v", val, string(data), res)
}
