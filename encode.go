package utcode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"math"
)

// Encode will encode the value using the default Encoder
func Encode(v interface{}) ([]byte, error) {
	var e Encoder
	err := e.Encode(v)
	if err != nil {
		return nil, err
	}
	return e.Bytes(), nil
}

// Encoder contains the output buffer of the value encoded
// and allows to register custom type encoders
type Encoder struct {
	bytes.Buffer
	custom map[reflect.Kind]typeEncoder
}

func NewEncoder() *Encoder {
	e := &Encoder{
		custom: make(map[reflect.Kind]typeEncoder),
	}
	return e
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

	e.WriteString("ut:")
	e.encodeType(value)

	return nil
}

// Register a custom type encoder
func (e *Encoder) Register(t reflect.Kind, encoder typeEncoder) {
	e.custom[t] = encoder
}

func (e *Encoder) encodeType(v reflect.Value) {
	encoder := e.typeEncoder(v.Kind())
	if encoder == nil {
		if v.IsValid() {
			panic(fmt.Errorf("unsupported encode type %v", v.Kind()))
		} else {
			e.WriteString("n:e")
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
		encoder, ok := e.custom[t]
		if !ok {
			return nil
		}
		return encoder
	}
}

type typeEncoder func(e *Encoder, v reflect.Value)

func boolEncoder(e *Encoder, v reflect.Value) {
	e.WriteString("b:")
	if v.Bool() {
		e.WriteString("1")
	} else {
		e.WriteString("0")
	}
}

func intEncoder(e *Encoder, v reflect.Value) {
	e.WriteString(fmt.Sprintf("i:%ve", v.Int()))
}

func uintEncoder(e *Encoder, v reflect.Value) {
	e.WriteString(fmt.Sprintf("i:%ve", v.Uint()))
}

func floatEncoder(e *Encoder, v reflect.Value) {
	var result string
	f := v.Float()
	if f == math.Floor(f) {
		result = fmt.Sprintf("i:%ve", f)
	} else {
		result = fmt.Sprintf("f:%vz", f)
	}
	e.WriteString(result)
}

func stringEncoder(e *Encoder, v reflect.Value) {
	b64 := base64.StdEncoding.EncodeToString([]byte(v.String()))
	e.WriteString(fmt.Sprintf("u%v:%v", len(b64), b64))
}

func structEncoder(e *Encoder, v reflect.Value) {
	e.WriteString("d:")

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

		e.WriteString(fmt.Sprintf("k%v:%v", len(name), name))
		e.encodeType(v.FieldByName(field.Name))
	}

	e.WriteString("e")
}

func mapEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.WriteString("n:e")
	}

	e.WriteString("d:")
	for _, k := range v.MapKeys() {
		if k.Type().Kind() != reflect.String {
			panic("map encoding supports only string as key")
		}

		str := k.String()
		e.WriteString(fmt.Sprintf("k%v:%v", len(str), str))

		e.encodeType(v.MapIndex(k))
	}
	e.WriteString("e")
}

var (
	bytesType = reflect.ValueOf([]byte{}).Type()
)

func sliceEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.WriteString("n:e")
		return
	}

	if v.Type() == bytesType {
		stringEncoder(e, reflect.ValueOf(string(v.Bytes())))
		return
	}

	e.WriteString("l:")
	for i := 0; i < v.Len(); i++ {
		e.encodeType(v.Index(i))
	}
	e.WriteString("e")
}

func ptrEncoder(e *Encoder, v reflect.Value) {
	if v.IsNil() {
		e.WriteString("n:e")
		return
	}

	e.encodeType(v.Elem())
}
