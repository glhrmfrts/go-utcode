package utcode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
)

// Encode will encode the value and return a byte slice
func Encode(v interface{}) ([]byte, error) {
	e := Encoder{dest: bytes.NewBuffer([]byte{})}
	err := e.Encode(v)

	if err != nil {
		return nil, err
	}

	bytesBuf := e.dest.(*bytes.Buffer)
	return bytesBuf.Bytes(), nil
}

type Encoder struct {
	dest io.Writer
}

func NewEncoder(dest io.Writer) *Encoder {
	return &Encoder{dest}
}

// Encode the value to utcode, returns an error if there's any
func (e *Encoder) Encode(v interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			if s, ok := r.(string); ok {
				panic(s)
			}
			err = r.(error)
		}
	}()

	value := reflect.ValueOf(v)

	e.dest.Write([]byte("ut:"))
	e.encodeType(value)

	return nil
}

func (e *Encoder) encodeType(v reflect.Value) {
	encoder := e.typeEncoder(v.Kind())
	if encoder == nil {
		if v.IsValid() {
			panic(fmt.Errorf("unsupported encode type %v", v.Kind()))
		} else {
			e.dest.Write([]byte("n:e"))
		}

		return
	}

	encoder(e, v)
}

func (e *Encoder) typeEncoder(t reflect.Kind) typeEncoder {
	switch t {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.Float32, reflect.Float64:
		return floatEncoder
	case reflect.String:
		return stringEncoder
	case reflect.Struct:
		return structEncoder
	case reflect.Map:
		return mapEncoder
	case reflect.Slice:
		return sliceEncoder
	case reflect.Array:
		return sliceEncoder
	case reflect.Ptr, reflect.Interface:
		return ptrEncoder
	default:
		return nil
	}
}

type typeEncoder func(e *Encoder, v reflect.Value)

func boolEncoder(e *Encoder, v reflect.Value) {
	result := []byte("b:")
	if v.Bool() {
		result = append(result, '0')
	} else {
		result = append(result, '0')
	}

	e.dest.Write(result)
}

func intEncoder(e *Encoder, v reflect.Value) {
	e.dest.Write([]byte(fmt.Sprintf("i:%ve", v.Int())))
}

func uintEncoder(e *Encoder, v reflect.Value) {
	e.dest.Write([]byte(fmt.Sprintf("i:%ve", v.Uint())))
}

func floatEncoder(e *Encoder, v reflect.Value) {
	e.dest.Write([]byte(fmt.Sprintf("f:%vz", v.Float())))
}

func stringEncoder(e *Encoder, v reflect.Value) {
	b64 := base64.StdEncoding.EncodeToString([]byte(v.String()))
	e.dest.Write([]byte(fmt.Sprintf("u%v:%v", len(b64), b64)))
}

func structEncoder(e *Encoder, v reflect.Value) {
	e.dest.Write([]byte("d:"))

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name, tag := field.Name, field.Tag.Get(TagName)
		if tag == "" {
			name = strings.ToLower(name[:1]) + name[1:]
		} else {
			name = tag
		}

		e.dest.Write([]byte(fmt.Sprintf("k%v:%v", len(name), name)))
		e.encodeType(v.FieldByName(field.Name))
	}

	e.dest.Write([]byte("e"))
}

func mapEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.dest.Write([]byte("n:e"))
	}

	e.dest.Write([]byte("d:"))
	for _, k := range v.MapKeys() {
		if k.Type().Kind() != reflect.String {
			panic("map encoding supports only string as key")
		}

		str := k.String()
		e.dest.Write([]byte(fmt.Sprintf("k%v:%v", len(str), str)))

		e.encodeType(v.MapIndex(k))
	}
	e.dest.Write([]byte("e"))
}

var (
	bytesType = reflect.ValueOf([]byte{}).Type()
)

func sliceEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.dest.Write([]byte("n:e"))
		return
	}

	if v.Type() == bytesType {
		stringEncoder(e, reflect.ValueOf(string(v.Bytes())))
		return
	}

	e.dest.Write([]byte("l:"))
	for i := 0; i < v.Len(); i++ {
		e.encodeType(v.Index(i))
	}
	e.dest.Write([]byte("e"))
}

func ptrEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.dest.Write([]byte("n:e"))
		return
	}

	e.encodeType(v.Elem())
}
