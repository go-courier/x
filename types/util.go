package types

import (
	"bytes"
	"fmt"
	"go/types"
	"reflect"
	"strconv"
	"strings"
)

func typeString(t Type) string {
	if pkgPath := t.PkgPath(); pkgPath != "" {
		return pkgPath + "." + t.Name()
	}

	switch k := t.Kind(); k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return k.String()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return k.String()
	case reflect.Bool:
		return k.String()
	case reflect.Float32, reflect.Float64:
		return k.String()
	case reflect.Slice:
		return "[]" + t.Elem().String()
	case reflect.Array:
		return fmt.Sprintf("[%d]", t.Len()) + t.Elem().String()
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", t.Key(), t.Elem())
	case reflect.Chan:
		return "chan " + t.Elem().String()
	case reflect.Struct:
		buf := bytes.NewBuffer(nil)
		buf.WriteString("struct {")
		n := t.NumField()
		for i := 0; i < n; i++ {
			buf.WriteRune(' ')
			f := t.Field(i)
			if !f.Anonymous() {
				buf.WriteString(f.Name())
				buf.WriteRune(' ')
			}
			buf.WriteString(f.Type().String())

			tag := f.Tag()
			if tag != "" {
				buf.WriteRune(' ')
				buf.WriteString(strconv.Quote(string(tag)))
			}

			if i == n-1 {
				buf.WriteRune(' ')
			} else {
				buf.WriteRune(';')
			}
		}
		buf.WriteString("}")
		return buf.String()
	case reflect.Interface:
		if name := t.Name(); name == "error" {
			return name
		}

		buf := bytes.NewBuffer(nil)
		buf.WriteString("interface {")
		n := t.NumMethod()
		for i := 0; i < n; i++ {
			buf.WriteRune(' ')
			m := t.Method(i)

			pkgPath := m.PkgPath()
			if pkgPath != "" {
				pkg := NewPackage(pkgPath)
				buf.WriteString(pkg.Name())
				buf.WriteRune('.')
			}

			buf.WriteString(m.Name())
			buf.WriteString(m.Type().String()[4:])

			if i == n-1 {
				buf.WriteRune(' ')
			} else {
				buf.WriteRune(';')
			}
		}
		buf.WriteString("}")
		return buf.String()
	case reflect.Func:
		buf := bytes.NewBuffer(nil)
		buf.WriteString("func(")
		{
			n := t.NumIn()
			for i := 0; i < n; i++ {
				p := t.In(i)

				if i == n-1 && t.IsVariadic() {
					buf.WriteString("...")
					buf.WriteString(p.Elem().String())
				} else {
					buf.WriteString(p.String())
				}

				if i < n-1 {
					buf.WriteString(", ")
				}
			}
			buf.WriteString(")")
		}

		{
			n := t.NumOut()
			if n > 0 {
				buf.WriteRune(' ')
			}
			if n > 1 {
				buf.WriteString("(")
			}
			for i := 0; i < n; i++ {
				if i > 0 {
					buf.WriteString(", ")
				}

				r := t.Out(i)
				buf.WriteString(r.String())
			}
			if n > 1 {
				buf.WriteString(")")
			}
		}

		return buf.String()
	}

	return t.Name()
}

func TypeFor(id string) types.Type {
	if v, ok := typesCache.Load(id); ok {
		return v.(types.Type)
	}
	t := typeFor(id)
	typesCache.Store(id, t)
	return t
}

var basicTypes = map[string]types.Type{}

func initBasicTypes() {
	for _, b := range types.Typ {
		basicTypes[types.TypeString(b, nil)] = b
	}

	basicTypes["interface {}"] = types.NewInterfaceType(nil, nil)
	basicTypes["error"] = NewPackage("errors").Scope().Lookup("New").Type().Underlying().(*types.Signature).Results().At(0).Type()
}

func typeFor(id string) types.Type {
	if len(basicTypes) == 0 {
		initBasicTypes()
	}

	if id == "" {
		return types.Typ[types.Invalid]
	}

	if t, ok := basicTypes[id]; ok {
		return t
	}

	if l := strings.Index(id, "map["); l == 0 {
		r := strings.Index(id, "]")
		return types.NewMap(TypeFor(id[4:r]), TypeFor(id[r+1:]))
	}

	l := strings.Index(id, "[")
	switch l {
	case 0:
		r := strings.Index(id, "]")
		if r-1 == l {
			return types.NewSlice(TypeFor(id[r+1:]))
		}
		n, _ := strconv.ParseInt(id[1:r], 10, 64)
		return types.NewArray(TypeFor(id[r+1:]), n)
	case -1:
		if i := strings.LastIndex(id, "."); i > 0 {
			importPath := id[0:i]
			name := id[i+1:]

			if pkg := NewPackage(importPath); pkg != nil {
				found := pkg.Scope().Lookup(name)
				if found != nil {
					return found.Type()
				}
			}
		}
	default:
		r := strings.Index(id, "]")
		fullPath := id[0:l]
		tparamNames := strings.Split(id[l+1:r], ",")

		if i := strings.LastIndex(fullPath, "."); i > 0 {
			importPath := fullPath[0:i]
			name := fullPath[i+1:]

			if pkg := NewPackage(importPath); pkg != nil {
				found := pkg.Scope().Lookup(name)
				if found != nil {
					named := &(*found.(*types.TypeName).Type().(*types.Named))

					typeParams := named.TypeParams()

					if n := typeParams.Len(); n > 0 {
						tparams := make([]*types.TypeParam, n)

						for i := 0; i < n; i++ {
							tparams[i] = types.NewTypeParam(typeParams.At(i).Obj(), TypeFor(tparamNames[i]))
						}

						named.SetTypeParams(tparams)
					}

					return found.Type()
				}
			}
		}
	}

	return types.Typ[types.Invalid]
}
