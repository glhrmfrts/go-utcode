package utcode

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

// Decode will decode the UTCode data using the default Decoder
func Decode(data []byte, v interface{}) error {
	d := Decoder{reader: strings.NewReader(data)}
	return d.Decode(v)
}

type Decoder struct {
	reader io.Reader
}

func NewDecoder(r io.Reader) {
	return &Decoder{
		reader: r,
	}
}

func (d *Decoder) Decode(v interface{}) (err error) {
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

	if !d.readAndMatch(3, "ut:") {
		panic(NewDecodeError("invalid utcode"))
	}

	if value.IsNil() {
		value.Set(d.decodeTypeAndCreate())
	} else {
		d.decodeType(value)
	}
	return nil
}

func (d *Decoder) decodeType(v reflect.Value) {
	if d.off >= len(d.data) {
		return
	}

	key, ok := d.readUntil(':')
	if !ok {
		panic(NewDecodeError("invalid utcode"))
	}
	d.read(1)

	decoder := d.typeDecoder(key)
	if decoder == nil {
		panic(NewDecodeError(fmt.Sprintf("invalid utcode type '%s'", d.peek())))
	}

	decoder(d, key, v)
}

// TODO: no-structs

func (d *Decoder) decodeTypeAndCreate() reflect.Value {
	if d.off >= len(d.data) {
		return reflect.ValueOf(nil)
	}

	key, ok := d.readUntil(':')
	if !ok {
		panic(NewDecodeError("invalid utcode"))
	}
	d.read(1)

	decoder, zeroValue := d.typeDecoderAndCreate(key)
	if decoder == nil {
		panic(NewDecodeError(fmt.Sprintf("invalid utcode type '%s'", d.peek())))
	}

	val := reflect.ValueOf(zeroValue)
	decoder(d, key, val)
	return val
}

func (d *Decoder) peek() byte {
	return d.data[d.off]
}

func (d *Decoder) read(n int) string {
	i := d.off
	str := string(d.data[i : i+n])
	d.off += n
	return str
}

func (d *Decoder) readUntil(ch byte) (string, bool) {
	var count int

	for i := d.off; i < len(d.data); i++ {
		if d.data[i] == ch {
			count = i - d.off
			break
		}
	}

	if count == 0 {
		return "", false
	}

	return d.read(count), true
}

func (d *Decoder) typeDecoder(key string) typeDecoder {
	switch key[0] {
	case 'n':
		return nilDecoder
	case 'b':
		return boolDecoder
	case 'i':
		return intDecoder
	case 'f':
		return floatDecoder
	case 's':
		return stringDecoder
	case 'u':
		return unicodeDecoder
	case 'd':
		return dictDecoder
	case 'l':
		return listDecoder
	case 'c':
		// TODO: will custom be prefixed with 'c'?
		return customDecoder
	default:
		return nil
	}
}

func (d *Decoder) typeDecoderAndCreate(key string) (typeDecoder, interface{}) {
	switch key[0] {
	case 'n':
		return nilDecoder, nil
	case 'b':
		val := false
		return boolDecoder, &val
	case 'i':
		val := 0
		return intDecoder, &val
	case 'f':
		val := 0.0
		return floatDecoder, &val
	case 's':
		val := ""
		return stringDecoder, &val
	case 'u':
		val := ""
		return unicodeDecoder, &val
	case 'd':
		return dictDecoder, &map[string]interface{}{}
	case 'l':
		return listDecoder, &[]interface{}{}
	case 'c':
		panic(NewDecodeError("custom type must be top-level"))
		fallthrough
	default:
		return nil, nil
	}
}

type DecodeError struct {
	what string
}

func NewDecodeError(what string) *DecodeError {
	return &DecodeError{
		what: what,
	}
}

func (d *DecodeError) Error() string {
	return d.what
}

type typeDecoder func(d *Decoder, key string, v reflect.Value)

func nilDecoder(d *Decoder, key string, v reflect.Value) {
	d.read(1)
}

func boolDecoder(d *Decoder, key string, v reflect.Value) {
	v.Elem().SetBool(!(d.peek() == '0'))
	d.read(1)
}

func intDecoder(d *Decoder, key string, v reflect.Value) {
	str, ok := d.readUntil('e')
	if !ok {
		panic(NewDecodeError("could not find int end"))
	}

	v.Elem().SetInt(int64(parseInt(str)))
	d.read(1)
}

func floatDecoder(d *Decoder, key string, v reflect.Value) {
	str, ok := d.readUntil('z')
	if !ok {
		panic(NewDecodeError("could not find float end"))
	}

	if f, err := strconv.ParseFloat(str, 64); err != nil {
		panic(err)
	} else {
		v.Elem().SetFloat(f)
	}
	d.read(1)
}

func stringDecoder(d *Decoder, key string, v reflect.Value) {
	length := parseInt(key[1:])
	v.Elem().SetString(d.read(length))
}

func unicodeDecoder(d *Decoder, key string, v reflect.Value) {
	length := parseInt(key[1:])
	data, err := base64.StdEncoding.DecodeString(d.read(length))
	if err != nil {
		panic(err)
	}

	v.Elem().SetString(string(data))
}

func dictDecoder(d *Decoder, key string, v reflect.Value) {
	mapValue := v
	if v.IsNil() {
		mapValue = reflect.ValueOf(map[string]interface{}{})
	}

	switch mapValue.Type().Kind() {
	case reflect.Ptr, reflect.Interface:
		dictDecoder(d, key, mapValue.Elem())
		return
	case reflect.Map:
		fillMap(d, mapValue.Interface().(map[string]interface{}))
	case reflect.Struct:
		fillStruct(d, mapValue)
	}

	if v.IsNil() {
		v.Set(mapValue)
	}

	d.read(1)
}

func listDecoder(d *Decoder, key string, v reflect.Value) {
	if !isValidList(v) {
		return
	}

	sliceValue := v
	if v.Elem().IsNil() {
		sliceValue = reflect.ValueOf(&[]interface{}{})
	}

	if elemType := sliceValue.Type().Elem(); elemType.Kind() == reflect.Struct {
		fillStructSlice(d, sliceValue, elemType)
	} else {
		fillSlice(d, sliceValue)
	}

	if v.Elem().IsNil() {
		v.Elem().Set(sliceValue)
	}

	d.read(1)
}

func customDecoder(d *Decoder, key string, v reflect.Value) {
	// TODO: custom decoding
}

func acceptNil(v reflect.Kind) bool {
	return false
}

func dictKey(d *Decoder) (string, bool) {
	key, ok := d.readUntil(':')
	if !ok {
		return "", false
	}

	if key[0] != 'k' {
		return "", false
	}
	d.read(1)

	length := parseInt(key[1:])
	return d.read(length), true
}

func fillMap(d *Decoder, out map[string]interface{}) {
	for {
		if d.peek() == 'e' {
			break
		}

		key, ok := dictKey(d)
		if !ok {
			break
		}

		val := d.decodeTypeAndCreate()
		if val.IsValid() {
			out[key] = val.Elem().Interface()
		} else {
			out[key] = nil
		}
	}
}

func fillStruct(d *Decoder, v reflect.Value) {
	fields := structFieldsMap(v.Type())
	for {
		if d.peek() == 'e' {
			break
		}

		key, ok := dictKey(d)
		if !ok {
			break
		}

		field, ok := fields[key]
		if !ok {
			continue
		}

		setStructField(d, field, v)
	}
}

func parseInt(str string) int {
	if i, err := strconv.ParseInt(str, 0, 64); err != nil {
		panic(err)
		return 0
	} else {
		return int(i)
	}
}

func setStructField(d *Decoder, f *reflect.StructField, v reflect.Value) {
	kind := f.Type.Kind()
	switch kind {
	case reflect.Ptr:
		new := reflect.New(f.Type.Elem())
		d.decodeType(new)
		v.FieldByName(f.Name).Set(new)
	case reflect.Interface:
		val := d.decodeTypeAndCreate()
		v.FieldByName(f.Name).Set(val.Elem())
	default:
		d.decodeType(v.FieldByName(f.Name).Addr())
	}
}

func structFieldsMap(t reflect.Type) map[string]*reflect.StructField {
	res := make(map[string]*reflect.StructField)

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

		res[name] = &field
	}
	return res
}

func isValidList(v reflect.Value) bool {
	switch v.Type().Kind() {
	case reflect.Ptr:
		return true
	default:
		return false
	}
}

func fillSlice(d *Decoder, v reflect.Value) {
	length := v.Elem().Len()
	i := 0

	for d.peek() != 'e' {
		if i >= length {
			elem := d.decodeTypeAndCreate().Elem()
			v.Elem().Set(reflect.Append(v.Elem(), elem))
		} else {
			d.decodeType(v.Elem().Index(i).Addr())
		}

		i++
	}
}

func fillStructSlice(d *Decoder, v reflect.Value, elemType reflect.Type) {
	length := v.Len()
	i := 0

	for d.peek() != 'e' {
		if i >= length {
			zero := reflect.Zero(elemType)
			d.decodeType(zero)
			v.Elem().Set(reflect.Append(v.Elem(), zero))
		} else {
			d.decodeType(v.Elem().Index(i).Addr())
		}

		i++
	}
}
