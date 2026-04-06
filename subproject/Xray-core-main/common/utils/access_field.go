package utils

import (
	"reflect"
	"unsafe"
)

// AccessField can used to access unexported field of a struct
// valueType must be the exact type of the field or it will panic
func AccessField[valueType any](obj any, fieldName string) *valueType {
	field := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
	if field.Type() != reflect.TypeOf(*new(valueType)) {
		panic("field type: " + field.Type().String() + ", valueType: " + reflect.TypeOf(*new(valueType)).String())
	}
	v := (*valueType)(unsafe.Pointer(field.UnsafeAddr()))
	return v
}
