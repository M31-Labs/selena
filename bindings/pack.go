package bindings

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

// PackUniforms packs host values into layout's std140 uniform block.
//
// Values are keyed by descriptor field name. Scalars may be any Go integer or
// float type. Vectors and matrices may be slices or arrays of numeric values.
// Matrix values are expected in column-major order, matching GLSL/WGSL/Metal
// shader convention.
func PackUniforms(layout Layout, values map[string]any) ([]byte, error) {
	return PackUniformBlock(layout.UniformBlock, values)
}

// PackUniformBlock packs host values into block's std140 byte layout.
func PackUniformBlock(block UniformBlock, values map[string]any) ([]byte, error) {
	return packUniformBlock(block, values, false)
}

// PackUniformsWithDefaults packs host values into layout's uniform block,
// filling omitted fields from descriptor defaults when available.
func PackUniformsWithDefaults(layout Layout, values map[string]any) ([]byte, error) {
	return PackUniformBlockWithDefaults(layout.UniformBlock, values)
}

// PackUniformBlockWithDefaults packs host values into block's std140 byte
// layout, filling omitted fields from descriptor defaults when available.
func PackUniformBlockWithDefaults(block UniformBlock, values map[string]any) ([]byte, error) {
	return packUniformBlock(block, values, true)
}

func packUniformBlock(block UniformBlock, values map[string]any, useDefaults bool) ([]byte, error) {
	out := make([]byte, block.Size)
	defaults := map[string]DefaultValue{}
	if useDefaults {
		for _, d := range block.Defaults {
			defaults[d.Name] = d
		}
	}
	for _, field := range block.Fields {
		value, ok := values[field.Name]
		if !ok {
			d, hasDefault := defaults[field.Name]
			if !hasDefault {
				return nil, fmt.Errorf("missing uniform %q", field.Name)
			}
			value = d.Values
		}
		floats, err := numericValues(value)
		if err != nil {
			return nil, fmt.Errorf("uniform %q: %w", field.Name, err)
		}
		if err := packField(out, field, floats); err != nil {
			return nil, fmt.Errorf("uniform %q: %w", field.Name, err)
		}
	}
	return out, nil
}

func packField(out []byte, field Field, values []float32) error {
	want, ok := componentCount(field.Type)
	if !ok {
		return fmt.Errorf("unsupported type %q", field.Type)
	}
	if len(values) != want {
		return fmt.Errorf("%s needs %d components, got %d", field.Type, want, len(values))
	}
	switch field.Type {
	case "mat3":
		for col := 0; col < 3; col++ {
			for row := 0; row < 3; row++ {
				writeFloat32(out, field.Offset+col*16+row*4, values[col*3+row])
			}
		}
	default:
		for i, v := range values {
			writeFloat32(out, field.Offset+i*4, v)
		}
	}
	return nil
}

func componentCount(t string) (int, bool) {
	switch t {
	case "float":
		return 1, true
	case "vec2":
		return 2, true
	case "vec3":
		return 3, true
	case "vec4":
		return 4, true
	case "mat3":
		return 9, true
	case "mat4":
		return 16, true
	default:
		return 0, false
	}
}

func numericValues(v any) ([]float32, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, fmt.Errorf("nil value")
	}
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f, err := numericValue(rv)
		if err != nil {
			return nil, err
		}
		return []float32{f}, nil
	case reflect.Array, reflect.Slice:
		out := make([]float32, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			f, err := numericValue(rv.Index(i))
			if err != nil {
				return nil, fmt.Errorf("component %d: %w", i, err)
			}
			out[i] = f
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", v)
	}
}

func numericValue(v reflect.Value) (float32, error) {
	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			return 0, fmt.Errorf("nil component")
		}
		v = v.Elem()
	}
	var f float64
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		f = v.Convert(reflect.TypeOf(float64(0))).Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f = float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f = float64(v.Uint())
	default:
		return 0, fmt.Errorf("unsupported component type %s", v.Kind())
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite value %v", f)
	}
	return float32(f), nil
}

func writeFloat32(out []byte, offset int, value float32) {
	binary.LittleEndian.PutUint32(out[offset:offset+4], math.Float32bits(value))
}
