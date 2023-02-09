package reflect_util

import "reflect"

func GetFields(t reflect.Type) []reflect.StructField {
	num := t.NumField()
	fields := make([]reflect.StructField, 0, num)
	for i := 0; i < num; i++ {
		fields = append(fields, t.Field(i))
	}
	return fields
}

func GetFieldValues(v reflect.Value) []reflect.Value {
	num := v.NumField()
	fields := make([]reflect.Value, 0, num)
	for i := 0; i < num; i++ {
		fields = append(fields, v.Field(i))
	}
	return fields
}
