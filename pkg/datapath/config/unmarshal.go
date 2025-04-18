// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package config

import (
	"fmt"
	"reflect"
)

// StructToMap converts an instance of a Go struct generated by [varsToStruct]
// into a map of configuration values to be passed to LoadCollection.
//
// Only struct members with a `config` tag are included. The tag value is used
// as the key in the map, and the map value is the runtime value of the member.
func StructToMap(obj any) (map[string]any, error) {
	toValue := reflect.ValueOf(obj)
	if toValue.Type().Kind() != reflect.Ptr {
		return nil, fmt.Errorf("%T is not a pointer to struct", obj)
	}

	if toValue.IsNil() {
		return nil, fmt.Errorf("nil pointer to %T", obj)
	}

	fields, err := structFields(toValue.Elem(), TagName, nil)
	if err != nil {
		return nil, err
	}

	vars := make(map[string]any, len(fields))
	for _, field := range fields {
		tag := field.Tag.Get(TagName)
		if tag == "" {
			return nil, fmt.Errorf("field %s has no tag", field.Name)
		}

		if vars[tag] != nil {
			return nil, fmt.Errorf("tag %s on field %s occurs multiple times in object", tag, field.Name)
		}

		vars[tag] = field.value.Interface()
	}

	return vars, nil
}

// structField represents a struct field containing a struct tag.
type structField struct {
	reflect.StructField
	value reflect.Value
}

// structFields recursively gathers all fields of a struct and its nested
// structs that are tagged with the given tag.
func structFields(structVal reflect.Value, tag string, visited map[reflect.Type]bool) ([]structField, error) {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}

	structType := structVal.Type()
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", structType)
	}

	if visited[structType] {
		return nil, fmt.Errorf("recursion on type %s", structType)
	}

	fields := make([]structField, 0, structType.NumField())
	for i := range structType.NumField() {
		field := structField{structType.Field(i), structVal.Field(i)}

		// If the field is tagged, gather it and move on.
		name := field.Tag.Get(tag)
		if name != "" {
			fields = append(fields, field)
			continue
		}

		// If the field does not have an ebpf tag, but is a struct or a pointer
		// to a struct, attempt to gather its fields as well.
		var v reflect.Value
		switch field.Type.Kind() {
		case reflect.Ptr:
			if field.Type.Elem().Kind() != reflect.Struct {
				continue
			}

			if field.value.IsNil() {
				return nil, fmt.Errorf("nil pointer to %s", structType)
			}

			// Obtain the destination type of the pointer.
			v = field.value.Elem()

		case reflect.Struct:
			// Reference the value's type directly.
			v = field.value

		default:
			continue
		}

		inner, err := structFields(v, tag, visited)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		fields = append(fields, inner...)
	}

	return fields, nil
}
