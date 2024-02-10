package helpers

import "reflect"

func OverrideStructure(target interface{}, override interface{}) {
	t := reflect.ValueOf(target).Elem()
	o := reflect.ValueOf(override).Elem()
	numField := o.NumField()
	for i := 0; i < numField; i++ {
		v := t.Field(i)
		switch v.Kind() {
		case reflect.Struct:
		default:
			if !o.Field(i).IsZero() {
				t.Field(i).Set(o.Field(i))
			}
		}
	}
}
